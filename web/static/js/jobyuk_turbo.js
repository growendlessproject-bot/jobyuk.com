/**
 * ==============================================================================
 * JobyUK Turbo SPA Navigation Engine (Production-Grade Hybrid SPA)
 * ==============================================================================
 * Features:
 * - 0ms instant page loads via smart hover/touch prefetching & LRU memory cache
 * - Smooth YouTube/GitHub-style emerald glowing top progress bar
 * - Seamless #main-content cross-fade transitions without full-page reloads
 * - Dynamic document title, canonical, and meta tag synchronization
 * - Back/Forward browser history management via HTML5 pushState
 * - Automatic script execution & page-specific controller re-initialization
 * - 100% SEO, Google Jobs crawler, and AdSense compatibility preserved
 * ==============================================================================
 */

(function () {
    'use strict';

    // Disable in non-browser environments or if pushState isn't supported
    if (typeof window === 'undefined' || !window.history || !window.history.pushState) {
        return;
    }

    const CONFIG = {
        mainSelector: '#main-content',
        cacheTTL: 60 * 1000, // 60 seconds in-memory cache
        cacheMaxEntries: 35,
        prefetchDelay: 45,   // ms debounce on hover before prefetching
        ignoredPatterns: [
            /\/api\//i,
            /\/pin-/i,
            /\/admin-lol/i,
            /\/wa-portal/i,
            /\/wa-client/i,
            /\/my-alerts/i,
            /\.(pdf|zip|png|jpe?g|svg|webp|ico|css|js|txt|xml|json)$/i
        ]
    };

    // In-memory page cache Map<url, { html: string, title: string, timestamp: number }>
    const pageCache = new Map();
    const scrollPositions = new Map();
    let currentRenderedUrl = cleanUrl(window.location.href);
    let currentPrefetchTimeout = null;
    let isNavigating = false;
    let progressBar = null;

    // ==========================================================================
    // 1. Top Loading Progress Bar Controller
    // ==========================================================================
    function getProgressBar() {
        if (!progressBar) {
            progressBar = document.getElementById('jy-turbo-progress');
            if (!progressBar) {
                progressBar = document.createElement('div');
                progressBar.id = 'jy-turbo-progress';
                progressBar.setAttribute('aria-hidden', 'true');
                document.body.appendChild(progressBar);
            }
        }
        return progressBar;
    }

    let progressTimer = null;
    let progressValue = 0;

    function startProgress() {
        const bar = getProgressBar();
        if (!bar) return;

        if (progressTimer) clearInterval(progressTimer);
        progressValue = 18;
        bar.style.opacity = '1';
        bar.style.width = '18%';

        progressTimer = setInterval(() => {
            if (progressValue < 82) {
                // Smoothly decelerate progress as it approaches 85%
                const increment = Math.max(1, (85 - progressValue) * 0.15);
                progressValue += increment;
                bar.style.width = progressValue + '%';
            }
        }, 120);
    }

    function completeProgress() {
        const bar = getProgressBar();
        if (!bar) return;

        if (progressTimer) {
            clearInterval(progressTimer);
            progressTimer = null;
        }

        bar.style.width = '100%';
        setTimeout(() => {
            bar.style.opacity = '0';
            setTimeout(() => {
                bar.style.width = '0%';
            }, 250);
        }, 160);
    }

    function failProgress() {
        const bar = getProgressBar();
        if (!bar) return;
        if (progressTimer) {
            clearInterval(progressTimer);
            progressTimer = null;
        }
        bar.style.width = '100%';
        bar.style.backgroundColor = '#EF4444';
        setTimeout(() => {
            bar.style.opacity = '0';
            setTimeout(() => {
                bar.style.width = '0%';
                bar.style.backgroundColor = '';
            }, 250);
        }, 300);
    }

    // ==========================================================================
    // 2. URL Helpers & Filter Validator
    // ==========================================================================
    function cleanUrl(urlStr) {
        try {
            const u = new URL(urlStr, window.location.origin);
            return u.origin + u.pathname + u.search;
        } catch (e) {
            return urlStr;
        }
    }

    function isEligibleAnchor(anchor) {
        if (!anchor || anchor.tagName !== 'A') return false;

        const href = anchor.getAttribute('href');
        if (!href) return false;

        // Skip anchor-only or empty links
        if (href.startsWith('#') || href.startsWith('javascript:')) return false;

        // Skip special protocols
        if (href.startsWith('mailto:') || href.startsWith('tel:') || href.startsWith('whatsapp:')) return false;

        // Skip non-SPA attributes
        if (anchor.hasAttribute('download') || anchor.getAttribute('target') === '_blank') return false;
        if (anchor.getAttribute('data-no-spa') === 'true' || anchor.getAttribute('data-turbo') === 'false') return false;

        try {
            const destUrl = new URL(href, window.location.origin);
            // Must be same origin (JobyUK)
            if (destUrl.origin !== window.location.origin) return false;

            // Check ignored patterns
            const path = destUrl.pathname;
            for (let i = 0; i < CONFIG.ignoredPatterns.length; i++) {
                if (CONFIG.ignoredPatterns[i].test(path)) return false;
            }

            return true;
        } catch (e) {
            return false;
        }
    }

    // ==========================================================================
    // 3. Cache & Background Prefetching (0ms Instant Load)
    // ==========================================================================
    function getCachedPage(url) {
        const key = cleanUrl(url);
        const item = pageCache.get(key);
        if (!item) return null;

        if (Date.now() - item.timestamp > CONFIG.cacheTTL) {
            pageCache.delete(key);
            return null;
        }
        return item;
    }

    function setCachedPage(url, html, title) {
        const key = cleanUrl(url);
        // Manage LRU cache size
        if (pageCache.size >= CONFIG.cacheMaxEntries) {
            const oldestKey = pageCache.keys().next().value;
            pageCache.delete(oldestKey);
        }
        pageCache.set(key, {
            html,
            title,
            timestamp: Date.now()
        });
    }

    async function fetchPage(url) {
        const cached = getCachedPage(url);
        if (cached) return cached;

        const res = await fetch(url, {
            headers: {
                'X-Requested-With': 'JobyUK-Turbo',
                'Accept': 'text/html,application/xhtml+xml,application/xml'
            }
        });

        if (!res.ok) {
            throw new Error(`HTTP ${res.status}`);
        }

        const html = await res.text();
        const parser = new DOMParser();
        const doc = parser.parseFromString(html, 'text/html');
        const title = doc.title || document.title;

        setCachedPage(url, html, title);
        return { html, title, timestamp: Date.now() };
    }

    const prefetchedUrls = new Set();

    function prefetchUrl(url) {
        if (!url || getCachedPage(url)) return;
        try {
            const dest = new URL(url, window.location.origin);
            const clean = cleanUrl(dest.href);
            // Don't prefetch current URL or already-attempted URL
            if (clean === cleanUrl(window.location.href)) return;
            if (prefetchedUrls.has(clean)) return;
            prefetchedUrls.add(clean);

            fetchPage(dest.href).catch(() => {});
        } catch (e) {}
    }

    // ==========================================================================
    // 4. Core Navigation & DOM Swapper
    // ==========================================================================
    async function navigateTo(targetUrl, options = { pushState: true, scrollToTop: true, isPopState: false }) {
        if (isNavigating) {
            if (options.isPopState) {
                // If popstate arrives while already navigating, override previous navigation
                isNavigating = false;
            } else {
                return;
            }
        }

        const targetClean = cleanUrl(targetUrl);

        // If NOT a popstate navigation and target is identical to currently rendered page (without hash), just scroll top
        if (!options.isPopState && currentRenderedUrl === targetClean && !targetUrl.includes('#')) {
            window.scrollTo({ top: 0, behavior: 'smooth' });
            return;
        }

        // Save current scroll position before leaving
        scrollPositions.set(currentRenderedUrl, window.scrollY);

        isNavigating = true;
        startProgress();

        // Close mobile drawer if open
        if (typeof window.closeMobileDrawer === 'function') {
            window.closeMobileDrawer();
        }

        const mainEl = document.querySelector(CONFIG.mainSelector);
        if (mainEl) {
            mainEl.classList.add('jy-turbo-exiting');
        }

        try {
            const data = await fetchPage(targetUrl);
            const parser = new DOMParser();
            const newDoc = parser.parseFromString(data.html, 'text/html');
            const newMain = newDoc.querySelector(CONFIG.mainSelector);

            if (!newMain) {
                // If template structure doesn't match, fallback safely to standard navigation
                window.location.href = targetUrl;
                return;
            }

            // 1. Update Document Title
            document.title = newDoc.title || data.title;

            // 2. Synchronize Canonical URL & Robots
            syncMetaTag(newDoc, 'link[rel="canonical"]', 'href');
            syncMetaTag(newDoc, 'meta[name="description"]', 'content');
            syncMetaTag(newDoc, 'meta[name="robots"]', 'content');

            // 3. Update History & Current Rendered URL
            if (options.pushState) {
                window.history.pushState({ turbo: true, url: targetUrl }, document.title, targetUrl);
            }
            currentRenderedUrl = targetClean;

            // 4. Update Header Nav Active States
            updateActiveNavLinks(targetUrl);

            // 5. Swap Content with Smooth Entrance Animation
            if (mainEl) {
                mainEl.innerHTML = newMain.innerHTML;
                mainEl.classList.remove('jy-turbo-exiting');
                mainEl.classList.add('jy-turbo-entering');

                // Force layout reflow
                void mainEl.offsetWidth;

                requestAnimationFrame(() => {
                    mainEl.classList.remove('jy-turbo-entering');
                });
            }

            // 6. Handle Scroll Position
            if (targetUrl.includes('#')) {
                const hashId = targetUrl.split('#')[1];
                const targetElement = document.getElementById(hashId);
                if (targetElement) {
                    targetElement.scrollIntoView({ behavior: 'smooth' });
                }
            } else if (options.isPopState) {
                const savedY = scrollPositions.get(targetClean);
                if (typeof savedY === 'number') {
                    window.scrollTo({ top: savedY, behavior: 'instant' });
                }
            } else if (options.scrollToTop !== false) {
                window.scrollTo({ top: 0, behavior: 'instant' });
            }

            // 7. Execute Any Inline Scripts in the New Page
            executeNewScripts(mainEl);

            // 8. Trigger Page Controllers
            reinitializePageControllers();

            // 9. Fire Global Turbo Load Event
            window.dispatchEvent(new CustomEvent('jy:turbo-load', {
                detail: { url: targetUrl, title: document.title }
            }));

            // 10. Fire Analytics Pageview if available
            trackAnalytics(targetUrl);

            completeProgress();
        } catch (err) {
            console.warn('[JobyUK Turbo] Navigation fallback due to:', err);
            failProgress();
            // Seamless standard fallback
            window.location.href = targetUrl;
        } finally {
            isNavigating = false;
        }
    }

    // ==========================================================================
    // 5. Script Re-Execution & Controller Initializers
    // ==========================================================================
    function executeNewScripts(container) {
        if (!container) return;
        const scripts = container.querySelectorAll('script');
        scripts.forEach(oldScript => {
            try {
                const newScript = document.createElement('script');
                Array.from(oldScript.attributes).forEach(attr => {
                    newScript.setAttribute(attr.name, attr.value);
                });
                newScript.textContent = oldScript.textContent;
                if (oldScript.parentNode) {
                    oldScript.parentNode.replaceChild(newScript, oldScript);
                }
            } catch (scriptErr) {
                console.warn('[JobyUK Turbo] Script execution skipped:', scriptErr);
            }
        });
    }

    let adRefreshTimeout = null;
    let lastAdRefreshTimestamp = 0;
    const AD_REFRESH_MIN_INTERVAL = 3000; // 3 seconds minimum between ad requests to protect against invalid activity/spam

    function refreshAdsForNewPage() {
        if (adRefreshTimeout) {
            clearTimeout(adRefreshTimeout);
            adRefreshTimeout = null;
        }

        // Wait 350ms after page transition settles before requesting new ads.
        // If user rapidly clicks another link, intermediate request is cancelled cleanly.
        adRefreshTimeout = setTimeout(() => {
            const now = Date.now();
            if (now - lastAdRefreshTimestamp < AD_REFRESH_MIN_INTERVAL) {
                return;
            }
            lastAdRefreshTimestamp = now;

            // 1. WeForAds / Header Bidding programmatic ad refresh
            if (typeof window.JobyUKRefreshAds === 'function') {
                try {
                    window.JobyUKRefreshAds();
                } catch (e) {}
            } else if (window.wfa && typeof window.wfa.refresh === 'function') {
                try {
                    window.wfa.refresh();
                } catch (e) {}
            }

            // 2. Google AdSense (Auto Ads & Manual slot push)
            if (window.adsbygoogle) {
                try {
                    const uninitializedSlots = document.querySelectorAll('ins.adsbygoogle:not([data-adsbygoogle-status])');
                    uninitializedSlots.forEach(() => {
                        (window.adsbygoogle = window.adsbygoogle || []).push({});
                    });
                } catch (e) {}
            }

            // 3. Google Publisher Tag (GPT) if active
            if (window.googletag && window.googletag.pubadsReady && typeof window.googletag.pubads === 'function') {
                try {
                    window.googletag.pubads().refresh();
                } catch (e) {}
            }
        }, 350);
    }

    function reinitializePageControllers() {
        // Re-run Saved Jobs Dynamic Controller if on saved jobs page
        if (typeof window.renderSavedJobsList === 'function') {
            try { window.renderSavedJobsList(); } catch (e) {}
        }

        // Re-run Salary Calculator if on salary calculator page
        if (typeof window.initSalaryCalculator === 'function') {
            try { window.initSalaryCalculator(); } catch (e) {}
        }

        // Re-check saved job bookmarks on page
        if (typeof window.JobyUKSaved !== 'undefined' && typeof window.JobyUKSaved.syncButtons === 'function') {
            try { window.JobyUKSaved.syncButtons(); } catch (e) {}
        }

        // Re-initialize Job & Location search autocomplete if on /jobs/
        if (typeof window.setupJobSearchAutocomplete === 'function') {
            try { window.setupJobSearchAutocomplete(); } catch (e) {}
        }
        if (typeof window.setupJobSearchAutocompleteMobile === 'function') {
            try { window.setupJobSearchAutocompleteMobile(); } catch (e) {}
        }

        // Re-initialize Auto-Scroll & Load More controller
        if (typeof window.initJobyUKInfiniteLoader === 'function') {
            try { window.initJobyUKInfiniteLoader(); } catch (e) {}
        }

        // Safely re-initialize / refresh advertising on the newly loaded page
        refreshAdsForNewPage();
    }

    function syncMetaTag(newDoc, selector, attrName) {
        const currentMeta = document.querySelector(selector);
        const newMeta = newDoc.querySelector(selector);
        if (currentMeta && newMeta) {
            currentMeta.setAttribute(attrName, newMeta.getAttribute(attrName));
        } else if (!currentMeta && newMeta) {
            document.head.appendChild(newMeta.cloneNode(true));
        }
    }

    function updateActiveNavLinks(targetUrl) {
        try {
            const dest = new URL(targetUrl, window.location.origin);
            const path = dest.pathname;

            document.querySelectorAll('.jy-header__link, .jy-mob-nav-link, .jy-header__nav a').forEach(link => {
                const href = link.getAttribute('href');
                if (!href) return;
                const linkPath = new URL(href, window.location.origin).pathname;

                if (linkPath === path || (path !== '/' && linkPath !== '/' && path.startsWith(linkPath))) {
                    link.classList.add('active', 'is-active');
                } else {
                    link.classList.remove('active', 'is-active');
                }
            });
        } catch (e) {}
    }

    function trackAnalytics(url) {
        if (typeof window.gtag === 'function') {
            try {
                const dest = new URL(url, window.location.origin);
                window.gtag('event', 'page_view', {
                    page_title: document.title,
                    page_location: dest.href,
                    page_path: dest.pathname + dest.search
                });
            } catch (e) {}
        }
    }

    // ==========================================================================
    // 6. Global Event Listeners & Interaction Handlers
    // ==========================================================================
    // Click Interceptor
    document.addEventListener('click', (e) => {
        const anchor = e.target.closest('a');
        if (!anchor) return;

        if (!isEligibleAnchor(anchor)) return;

        // Respect opening in new tab via Cmd/Ctrl/Shift/Middle-click
        if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) return;

        const href = anchor.getAttribute('href');
        e.preventDefault();
        navigateTo(href, { pushState: true, scrollToTop: true });
    }, true);

    // Hover / Touchstart Smart Prefetcher
    document.addEventListener('mouseover', (e) => {
        const anchor = e.target.closest('a');
        if (!anchor || !isEligibleAnchor(anchor)) return;

        if (currentPrefetchTimeout) clearTimeout(currentPrefetchTimeout);
        currentPrefetchTimeout = setTimeout(() => {
            prefetchUrl(anchor.getAttribute('href'));
        }, CONFIG.prefetchDelay);
    }, { passive: true });

    document.addEventListener('touchstart', (e) => {
        const anchor = e.target.closest('a');
        if (!anchor || !isEligibleAnchor(anchor)) return;
        prefetchUrl(anchor.getAttribute('href'));
    }, { passive: true });

    // Back / Forward Browser History
    window.addEventListener('popstate', (e) => {
        navigateTo(window.location.href, { pushState: false, isPopState: true, scrollToTop: false });
    });

    // Form Submissions (for GET search queries)
    document.addEventListener('submit', (e) => {
        const form = e.target.closest('form');
        if (!form) return;

        const method = (form.getAttribute('method') || 'GET').toUpperCase();
        if (method !== 'GET') return;

        const action = form.getAttribute('action') || window.location.pathname;
        if (!action.startsWith('/') && !action.startsWith(window.location.origin)) return;

        // Skip non-SPA forms
        if (form.getAttribute('data-no-spa') === 'true') return;

        e.preventDefault();
        const formData = new FormData(form);
        const params = new URLSearchParams();
        for (const [key, val] of formData.entries()) {
            if (val.trim()) {
                params.append(key, val);
            }
        }

        const queryString = params.toString();
        const targetUrl = action + (queryString ? '?' + queryString : '');
        navigateTo(targetUrl, { pushState: true, scrollToTop: true });
    });

    // Initialize initial page state
    if (window.history.state === null) {
        window.history.replaceState({ turbo: true, url: window.location.href }, document.title, window.location.href);
    }

    // Expose programmatic navigation API on window
    window.JobyUKTurbo = {
        visit: navigateTo,
        prefetch: prefetchUrl,
        clearCache: () => pageCache.clear()
    };

    console.log('%c[JobyUK Turbo] SPA Engine initialized successfully ⚡', 'color: #0F5132; font-weight: bold;');
})();
