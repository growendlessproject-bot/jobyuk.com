/**
 * JobyUK Unified Auto-Scroll & "Load More" Engine
 * Supports:
 * - Jobs Feed: "Load More Jobs"
 * - Roles Directory: "Load More Roles"
 * - Salary Directory: "Load More Salaries"
 * Features:
 * - IntersectionObserver auto-scroll loading 450px before reaching bottom
 * - Interactive button click fallback with animated spinner
 * - DOM fragment extraction without full-page reloads
 * - Smooth fade-in card entrance
 * - History state URL synchronization
 * - Turbo SPA compatibility
 */
(function() {
    'use strict';

    function initJobyUKInfiniteLoader() {
        const wrap = document.querySelector('.jy-load-more-wrap, .jy-load-more-container');
        if (!wrap) return;

        // Reset if element re-created or not yet initialized
        if (wrap.dataset.infiniteInit === 'true') return;
        wrap.dataset.infiniteInit = 'true';

        const btn = wrap.querySelector('.jy-load-more-btn');
        const spinner = wrap.querySelector('.jy-load-more-spinner');
        const textEl = wrap.querySelector('.jy-load-more-text');
        const arrow = wrap.querySelector('.jy-load-more-arrow');

        let currentPage = parseInt(wrap.getAttribute('data-current-page'), 10) || 1;
        const totalPages = parseInt(wrap.getAttribute('data-total-pages'), 10) || 1;
        const pageType = wrap.getAttribute('data-type') || 'items'; // 'jobs', 'roles', 'salaries'

        if (currentPage >= totalPages) {
            wrap.style.display = 'none';
            return;
        }

        // Hide static pagination links when infinite loader is active
        const oldPagination = document.querySelector('.pagination-mode-wrap, .jy-pagination-wrap');
        if (oldPagination) {
            oldPagination.style.display = 'none';
        }

        let isLoading = false;
        let observer = null;

        const defaultLabel = textEl ? textEl.textContent.trim() : 'Load More';
        let loadingLabel = 'Loading More...';
        if (pageType === 'jobs') loadingLabel = 'Loading More Jobs...';
        else if (pageType === 'roles') loadingLabel = 'Loading More Roles...';
        else if (pageType === 'salaries') loadingLabel = 'Loading More Salaries...';

        async function loadNextPage() {
            if (isLoading || currentPage >= totalPages) return;

            // Pause auto-load if client-side live search is actively filtering
            const searchInput = document.querySelector('#salary-search-input, #role-search-input, #company-search');
            if (searchInput && searchInput.value.trim().length > 0) return;

            isLoading = true;
            if (spinner) spinner.style.display = 'inline-block';
            if (arrow) arrow.style.display = 'none';
            if (textEl) textEl.textContent = loadingLabel;
            if (btn) btn.disabled = true;

            const nextPage = currentPage + 1;
            const currentUrl = new URL(window.location.href);
            currentUrl.searchParams.set('page', nextPage);

            try {
                const res = await fetch(currentUrl.toString(), {
                    headers: { 'X-Requested-With': 'XMLHttpRequest' }
                });

                if (!res.ok) throw new Error('HTTP ' + res.status);

                const html = await res.text();
                const parser = new DOMParser();
                const doc = parser.parseFromString(html, 'text/html');

                let targetContainer = null;
                let newItems = [];

                if (pageType === 'jobs') {
                    targetContainer = document.querySelector('.listing-jobs, #jobs-container, .jy-dream-jobs-stack');
                    const sourceContainer = doc.querySelector('.listing-jobs, #jobs-container, .jy-dream-jobs-stack');
                    if (sourceContainer) {
                        newItems = Array.from(sourceContainer.children).filter(el => 
                            el.classList.contains('job-card') || 
                            el.classList.contains('jy-job-card') || 
                            el.classList.contains('job-card-link') ||
                            el.classList.contains('jy-dream-card') ||
                            el.classList.contains('jy-mob-job-card') ||
                            el.tagName.toLowerCase() === 'article' ||
                            el.tagName.toLowerCase() === 'a'
                        );
                    }
                } else if (pageType === 'roles') {
                    targetContainer = document.querySelector('#rolesGrid, .jy-roles-grid, #roles-cards-list-m');
                    const sourceContainer = doc.querySelector('#rolesGrid, .jy-roles-grid, #roles-cards-list-m');
                    if (sourceContainer) {
                        newItems = Array.from(sourceContainer.querySelectorAll('.jy-role-card, .jy-mob-role-card'));
                    }
                } else if (pageType === 'salaries') {
                    targetContainer = document.querySelector('#salaryGrid, .jy-salary-grid, #salary-cards-list-m');
                    const sourceContainer = doc.querySelector('#salaryGrid, .jy-salary-grid, #salary-cards-list-m');
                    if (sourceContainer) {
                        newItems = Array.from(sourceContainer.querySelectorAll('.jy-salary-card, .jy-mob-sal-card'));
                    }
                }

                if (targetContainer && newItems.length > 0) {
                    newItems.forEach(item => {
                        item.classList.add('jy-fade-in');
                        targetContainer.appendChild(item);
                    });

                    currentPage = nextPage;
                    wrap.setAttribute('data-current-page', currentPage);

                    // Update URL parameter in history
                    try {
                        window.history.replaceState(window.history.state, '', currentUrl.toString());
                    } catch (e) {}

                    // Re-sync bookmarks if jobs
                    if (typeof window.JobyUKSaved !== 'undefined' && typeof window.JobyUKSaved.syncButtons === 'function') {
                        window.JobyUKSaved.syncButtons();
                    }
                }

                if (currentPage >= totalPages) {
                    wrap.style.display = 'none';
                    if (observer) observer.disconnect();
                }
            } catch (err) {
                console.warn('[JobyUK Infinite] Load error:', err);
            } finally {
                isLoading = false;
                if (spinner) spinner.style.display = 'none';
                if (arrow) arrow.style.display = 'inline-block';
                if (textEl) textEl.textContent = defaultLabel;
                if (btn) btn.disabled = false;
            }
        }

        // 1. Manual Click Trigger
        if (btn) {
            btn.addEventListener('click', function(e) {
                e.preventDefault();
                loadNextPage();
            });
        }

        // 2. Auto-scroll on reach bottom via IntersectionObserver
        if ('IntersectionObserver' in window) {
            observer = new IntersectionObserver(function(entries) {
                entries.forEach(function(entry) {
                    if (entry.isIntersecting) {
                        loadNextPage();
                    }
                });
            }, {
                root: null,
                rootMargin: '450px 0px',
                threshold: 0.01
            });
            observer.observe(wrap);
        }

        // 3. Fallback window scroll listener
        let scrollThrottle = false;
        window.addEventListener('scroll', function() {
            if (scrollThrottle || isLoading || currentPage >= totalPages) return;
            if (wrap.style.display === 'none') return;

            scrollThrottle = true;
            setTimeout(function() {
                scrollThrottle = false;
                if (wrap.style.display === 'none') return;
                const rect = wrap.getBoundingClientRect();
                if (rect.top <= (window.innerHeight || document.documentElement.clientHeight) + 400) {
                    loadNextPage();
                }
            }, 120);
        }, { passive: true });
    }

    window.initJobyUKInfiniteLoader = initJobyUKInfiniteLoader;

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initJobyUKInfiniteLoader);
    } else {
        initJobyUKInfiniteLoader();
    }

    window.addEventListener('jy:turbo-load', initJobyUKInfiniteLoader);
})();
