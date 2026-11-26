document.addEventListener('DOMContentLoaded', () => {
    // ==========================================
    // Next-Level Mobile App Drawer Controller
    // ==========================================
    const menuToggle = document.getElementById('menu-toggle');
    const drawerOverlay = document.getElementById('jMobileDrawerOverlay') || document.getElementById('mobile-drawer-overlay');
    const drawerPanel = document.getElementById('jMobileDrawer');
    const drawerCloseBtn = document.getElementById('jMobileDrawerCloseBtn') || document.getElementById('mobile-drawer-close');
    const drawerBackdrop = document.getElementById('jMobileDrawerOverlay') || document.getElementById('mobile-drawer-backdrop');

    window.openMobileDrawer = function(e) {
        if (e && e.preventDefault) e.preventDefault();
        const overlay = document.getElementById('jMobileDrawerOverlay') || drawerOverlay;
        const drawer = document.getElementById('jMobileDrawer') || drawerPanel;
        if (!drawer) return;
        if (overlay) {
            overlay.classList.add('is-open');
            overlay.setAttribute('aria-hidden', 'false');
        }
        drawer.classList.add('is-open');
        drawer.setAttribute('aria-hidden', 'false');
        if (menuToggle) menuToggle.setAttribute('aria-expanded', 'true');
        document.body.classList.add('jy-drawer-open');
    };

    window.closeMobileDrawer = function(e) {
        if (e && e.preventDefault) e.preventDefault();
        const overlay = document.getElementById('jMobileDrawerOverlay') || drawerOverlay;
        const drawer = document.getElementById('jMobileDrawer') || drawerPanel;
        if (overlay) {
            overlay.classList.remove('is-open');
            overlay.setAttribute('aria-hidden', 'true');
        }
        if (drawer) {
            drawer.classList.remove('is-open');
            drawer.setAttribute('aria-hidden', 'true');
        }
        if (menuToggle) menuToggle.setAttribute('aria-expanded', 'false');
        document.body.classList.remove('jy-drawer-open');
    };

    if (menuToggle) {
        menuToggle.addEventListener('click', (e) => {
            e.preventDefault();
            window.openMobileDrawer(e);
        });
    }

    if (drawerCloseBtn) {
        drawerCloseBtn.addEventListener('click', (e) => {
            e.preventDefault();
            window.closeMobileDrawer(e);
        });
    }

    if (drawerBackdrop) {
        drawerBackdrop.addEventListener('click', window.closeMobileDrawer);
    }

    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            window.closeMobileDrawer();
        }
    });

    // Mobile Filters Toggle (collapsible drawer)
    const filterToggle = document.getElementById('filter-toggle');
    const filterDrawer = document.getElementById('filter-drawer');
    const filterOverlay = document.getElementById('filter-overlay');
    const closeFilters = document.getElementById('close-filters');

    if (filterToggle && filterDrawer && filterOverlay) {
        const toggleDrawer = (show) => {
            if (show) {
                filterDrawer.classList.add('filters-drawer--open');
                filterOverlay.classList.add('filters-overlay--visible');
            } else {
                filterDrawer.classList.remove('filters-drawer--open');
                filterOverlay.classList.remove('filters-overlay--visible');
            }
        };

        filterToggle.addEventListener('click', () => toggleDrawer(true));
        filterOverlay.addEventListener('click', () => toggleDrawer(false));
        if (closeFilters) {
            closeFilters.addEventListener('click', () => toggleDrawer(false));
        }
    }

    // Smart Header Scroll Controller
    const header = document.getElementById('site-header');

    window.addEventListener('scroll', () => {
        const currentScrollY = window.scrollY;

        // Header scroll sticky style transition
        if (header) {
            if (currentScrollY > 10) {
                header.classList.add('header--scrolled');
            } else {
                header.classList.remove('header--scrolled');
            }
        }
    }, { passive: true });

    // Indeed-style Autocomplete and Suggestions Logic
    const queryInput = document.getElementById('query-input');
    const queryDropdown = document.getElementById('query-dropdown');
    const queryField = document.getElementById('query-field');

    const locationInput = document.getElementById('location-input');
    const locationDropdown = document.getElementById('location-dropdown');
    const locationField = document.getElementById('location-field');
    const searchForm = queryInput ? queryInput.closest('form') : null;

    if (queryInput && queryDropdown && locationInput && locationDropdown) {
        let activeQueryIndex = -1;
        let activeLocationIndex = -1;

        const getVisibleItems = (dropdown) => {
            return Array.from(dropdown.querySelectorAll('.search-dropdown__item')).filter(
                item => item.style.display !== 'none'
            );
        };

        const updateActiveItem = (items, index, activeClass) => {
            items.forEach((item, i) => {
                if (i === index) {
                    item.classList.add(activeClass);
                    item.scrollIntoView({ block: 'nearest' });
                } else {
                    item.classList.remove(activeClass);
                }
            });
        };

        // Smart UK Jobs Relevancy Database
        const TOP_UK_ROLES = [
            "Software Engineer", "Frontend Developer", "Backend Developer", "Full Stack Developer", "DevOps Engineer",
            "Data Analyst", "Data Scientist", "Product Manager", "Project Manager", "Business Analyst",
            "Registered Nurse", "NHS Staff Nurse", "Healthcare Assistant", "Care Worker", "Dental Nurse",
            "Customer Service Advisor", "Call Centre Agent", "Receptionist", "Administrative Assistant",
            "Accountant", "Finance Assistant", "Bookkeeper", "Financial Analyst", "Payroll Officer",
            "Marketing Executive", "Digital Marketing Specialist", "Content Writer", "Graphic Designer",
            "Sales Executive", "Business Development Manager", "Retail Assistant", "Store Manager",
            "Warehouse Operative", "Delivery Driver", "Forklift Driver", "HGV Driver",
            "Teacher", "Teaching Assistant", "Tutor", "Early Years Practitioner",
            "Electrician", "Plumber", "Maintenance Engineer", "Civil Engineer", "Mechanical Engineer",
            "Human Resources Officer", "Recruitment Consultant", "Operations Manager"
        ];

        const TOP_UK_LOCATIONS = [
            "Remote, UK", "London, England", "Manchester, England", "Birmingham, England", "Leeds, England",
            "Glasgow, Scotland", "Edinburgh, Scotland", "Bristol, England", "Liverpool, England",
            "Sheffield, England", "Newcastle, England", "Nottingham, England", "Leicester, England",
            "Cardiff, Wales", "Belfast, Northern Ireland"
        ];

        // Query Suggestions with Relevancy Engine
        const filterQuerySuggestions = () => {
            const rawVal = queryInput.value.trim();
            const val = rawVal.toLowerCase();
            const listEl = queryDropdown.querySelector('.search-dropdown__list');
            const header = queryDropdown.querySelector('.search-dropdown__header');

            if (!listEl) return;

            let suggestions = [];

            if (val.length === 0) {
                if (header) header.textContent = 'Popular UK Searches';
                suggestions = [
                    "Software Engineer Remote", "NHS Nurse London", "Customer Service Manchester",
                    "Project Manager Birmingham", "Part Time Retail Leeds", "Data Analyst Edinburgh",
                    "Warehouse Assistant Bristol", "Accountant Glasgow"
                ];
            } else {
                if (header) header.textContent = 'Related Searches';

                // Add current exact search query first
                suggestions.push(rawVal);

                // Find matching roles from knowledge base
                const words = val.split(/\s+/).filter(w => w.length > 1);
                const matchedRoles = TOP_UK_ROLES.filter(r => {
                    const rLower = r.toLowerCase();
                    return words.some(w => rLower.includes(w)) && rLower !== val;
                }).slice(0, 5);

                matchedRoles.forEach(r => {
                    if (!suggestions.includes(r)) suggestions.push(r);
                });

                // Generate smart variations
                const baseWord = rawVal.replace(/\b(remote|london|manchester|full time|part time|senior|junior)\b/gi, '').trim();
                const variations = [
                    `Senior ${baseWord || rawVal}`,
                    `Junior ${baseWord || rawVal}`,
                    `${baseWord || rawVal} Remote`,
                    `${baseWord || rawVal} London`,
                    `${baseWord || rawVal} Manchester`,
                    `${baseWord || rawVal} (Full Time)`
                ];

                variations.forEach(v => {
                    if (v.trim() && !suggestions.some(s => s.toLowerCase() === v.toLowerCase()) && suggestions.length < 8) {
                        suggestions.push(v);
                    }
                });
            }

            if (suggestions.length === 0) {
                queryDropdown.style.display = 'none';
                return;
            }

            // Render suggestions
            listEl.innerHTML = suggestions.map(s => `
                <li class="search-dropdown__item" data-value="${escapeHTML(s)}">
                    <svg class="search-dropdown__item-icon" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                    </svg>
                    <span>${highlightMatch(s, val)}</span>
                </li>
            `).join('');

            queryDropdown.style.display = 'block';
            activeQueryIndex = -1;
            updateActiveItem(getVisibleItems(queryDropdown), -1, 'search-dropdown__item--active');
        };

        const highlightMatch = (text, query) => {
            if (!query) return escapeHTML(text);
            const regex = new RegExp(`(${query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi');
            return escapeHTML(text).replace(regex, '<strong>$1</strong>');
        };

        const escapeHTML = (str) => {
            const p = document.createElement('p');
            p.textContent = str;
            return p.innerHTML;
        };

        queryInput.addEventListener('focus', () => {
            if (locationDropdown) locationDropdown.style.display = 'none';
            filterQuerySuggestions();
        });

        queryInput.addEventListener('click', () => {
            if (locationDropdown) locationDropdown.style.display = 'none';
            filterQuerySuggestions();
        });

        queryInput.addEventListener('input', () => {
            if (locationDropdown) locationDropdown.style.display = 'none';
            filterQuerySuggestions();
        });

        queryInput.addEventListener('keydown', (e) => {
            const visibleItems = getVisibleItems(queryDropdown);
            if (queryDropdown.style.display === 'none') {
                if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
                    if (locationDropdown) locationDropdown.style.display = 'none';
                    filterQuerySuggestions();
                    return;
                }
            }

            if (e.key === 'ArrowDown') {
                e.preventDefault();
                if (visibleItems.length > 0) {
                    activeQueryIndex = (activeQueryIndex + 1) % visibleItems.length;
                    updateActiveItem(visibleItems, activeQueryIndex, 'search-dropdown__item--active');
                }
            } else if (e.key === 'ArrowUp') {
                e.preventDefault();
                if (visibleItems.length > 0) {
                    activeQueryIndex = (activeQueryIndex - 1 + visibleItems.length) % visibleItems.length;
                    updateActiveItem(visibleItems, activeQueryIndex, 'search-dropdown__item--active');
                }
            } else if (e.key === 'Enter') {
                if (activeQueryIndex >= 0 && visibleItems[activeQueryIndex]) {
                    e.preventDefault();
                    queryInput.value = visibleItems[activeQueryIndex].getAttribute('data-value');
                    queryDropdown.style.display = 'none';
                    if (searchForm) searchForm.submit();
                }
            } else if (e.key === 'Escape') {
                queryDropdown.style.display = 'none';
                activeQueryIndex = -1;
            }
        });

        // Location Autocomplete
        const filterLocationSuggestions = () => {
            const rawVal = locationInput.value.trim();
            const val = rawVal.toLowerCase();
            const listEl = locationDropdown.querySelector('.search-dropdown__list');

            if (!listEl) return;

            let suggestions = [];
            if (val.length === 0) {
                suggestions = TOP_UK_LOCATIONS;
            } else {
                suggestions = TOP_UK_LOCATIONS.filter(l => l.toLowerCase().includes(val));
                if (!suggestions.some(s => s.toLowerCase() === val)) {
                    suggestions.unshift(rawVal);
                }
            }

            if (suggestions.length === 0) {
                locationDropdown.style.display = 'none';
                return;
            }

            listEl.innerHTML = suggestions.map(loc => `
                <li class="search-dropdown__item" data-value="${escapeHTML(loc)}">
                    <svg class="search-dropdown__item-icon" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z" />
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 11a3 3 0 11-6 0 3 3 0 016 0z" />
                    </svg>
                    <span>${highlightMatch(loc, val)}</span>
                </li>
            `).join('');

            locationDropdown.style.display = 'block';
            activeLocationIndex = -1;
            updateActiveItem(getVisibleItems(locationDropdown), -1, 'search-dropdown__item--active');
        };

        locationInput.addEventListener('focus', () => {
            if (queryDropdown) queryDropdown.style.display = 'none';
            filterLocationSuggestions();
        });

        locationInput.addEventListener('click', () => {
            if (queryDropdown) queryDropdown.style.display = 'none';
            filterLocationSuggestions();
        });

        locationInput.addEventListener('input', () => {
            if (queryDropdown) queryDropdown.style.display = 'none';
            filterLocationSuggestions();
        });

        locationInput.addEventListener('keydown', (e) => {
            const visibleItems = getVisibleItems(locationDropdown);
            if (locationDropdown.style.display === 'none') {
                if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
                    if (queryDropdown) queryDropdown.style.display = 'none';
                    filterLocationSuggestions();
                    return;
                }
            }

            if (e.key === 'ArrowDown') {
                e.preventDefault();
                if (visibleItems.length > 0) {
                    activeLocationIndex = (activeLocationIndex + 1) % visibleItems.length;
                    updateActiveItem(visibleItems, activeLocationIndex, 'search-dropdown__item--active');
                }
            } else if (e.key === 'ArrowUp') {
                e.preventDefault();
                if (visibleItems.length > 0) {
                    activeLocationIndex = (activeLocationIndex - 1 + visibleItems.length) % visibleItems.length;
                    updateActiveItem(visibleItems, activeLocationIndex, 'search-dropdown__item--active');
                }
            } else if (e.key === 'Enter') {
                if (activeLocationIndex >= 0 && visibleItems[activeLocationIndex]) {
                    e.preventDefault();
                    locationInput.value = visibleItems[activeLocationIndex].getAttribute('data-value');
                    locationDropdown.style.display = 'none';
                    if (searchForm) searchForm.submit();
                }
            } else if (e.key === 'Escape') {
                locationDropdown.style.display = 'none';
                activeLocationIndex = -1;
            }
        });

        // Gesture-aware touch & drag handling for suggestions (allows smooth scrolling on mobile without accidental selection)
        const setupDropdownSelection = (dropdown, input) => {
            let startX = 0;
            let startY = 0;
            let isDragging = false;

            dropdown.addEventListener('pointerdown', (e) => {
                // Prevent input blur so focus/dropdown is not prematurely lost
                e.preventDefault();
                startX = e.clientX;
                startY = e.clientY;
                isDragging = false;
            });

            dropdown.addEventListener('pointermove', (e) => {
                const diffX = Math.abs(e.clientX - startX);
                const diffY = Math.abs(e.clientY - startY);
                if (diffX > 6 || diffY > 6) {
                    isDragging = true;
                }
            });

            dropdown.addEventListener('pointerup', (e) => {
                if (isDragging) {
                    // User was scrolling through the list, do not select item
                    return;
                }
                const item = e.target.closest('.search-dropdown__item');
                if (item) {
                    const val = item.getAttribute('data-value');
                    if (val !== null) {
                        input.value = val;
                        dropdown.style.display = 'none';
                        if (queryDropdown) queryDropdown.style.display = 'none';
                        if (locationDropdown) locationDropdown.style.display = 'none';
                        if (searchForm) {
                            setTimeout(() => searchForm.submit(), 30);
                        }
                    }
                }
            });
        };

        setupDropdownSelection(queryDropdown, queryInput);
        setupDropdownSelection(locationDropdown, locationInput);

        // Hide dropdowns when clicking outside
        document.addEventListener('pointerdown', (e) => {
            if (queryDropdown && !queryInput.contains(e.target) && !queryDropdown.contains(e.target)) {
                queryDropdown.style.display = 'none';
            }
            if (locationDropdown && !locationInput.contains(e.target) && !locationDropdown.contains(e.target)) {
                locationDropdown.style.display = 'none';
            }
        });
    }

    // --- Job Search Back State & Scroll Restoration ---
    const isDetail = window.location.pathname.startsWith('/jobs/') && window.location.pathname !== '/jobs/' && window.location.pathname !== '/jobs';

    // Helper to save job slug to viewed history in localStorage
    const saveJobAsViewed = (href) => {
        if (!href) return;
        const parts = href.split('/').filter(Boolean);
        const slug = parts[parts.length - 1];

        if (slug) {
            let viewed = [];
            try {
                viewed = JSON.parse(localStorage.getItem('viewedJobs')) || [];
            } catch (e) {
                viewed = [];
            }
            if (!Array.isArray(viewed)) viewed = [];

            if (!viewed.includes(slug)) {
                viewed.push(slug);
                if (viewed.length > 500) {
                    viewed.shift(); // keep history size reasonable
                }
                localStorage.setItem('viewedJobs', JSON.stringify(viewed));
            }
        }
    };

    // Helper to scan page for job cards and apply viewed styles
    const markViewedJobsOnPage = () => {
        let viewed = [];
        try {
            viewed = JSON.parse(localStorage.getItem('viewedJobs')) || [];
        } catch (e) {
            viewed = [];
        }
        if (!Array.isArray(viewed)) return;

        const cards = document.querySelectorAll('.job-card');
        cards.forEach(card => {
            const link = card.querySelector('.job-card__title a');
            if (!link) return;

            const href = link.getAttribute('href');
            if (!href) return;

            const parts = href.split('/').filter(Boolean);
            const slug = parts[parts.length - 1];

            if (slug && viewed.includes(slug)) {
                card.classList.add('job-card--viewed');

                const pillsContainer = card.querySelector('.job-card__pills');
                if (pillsContainer && !pillsContainer.querySelector('.job-pill--viewed')) {
                    const viewedPill = document.createElement('span');
                    viewedPill.className = 'job-pill job-pill--viewed';
                    viewedPill.innerHTML = `
                        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" style="margin-right: 3.5px; display: inline-block; vertical-align: middle;">
                            <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
                            <circle cx="12" cy="12" r="3"></circle>
                        </svg>Viewed`;
                    pillsContainer.appendChild(viewedPill);
                }
            }
        });
    };

    if (isDetail) {
        // We are on a job detail page: 
        // 1. Update the back link to point to the saved search URL
        const savedSearchUrl = sessionStorage.getItem('lastSearchUrl');
        const backLink = document.querySelector('.job-back-link');
        if (backLink && savedSearchUrl) {
            backLink.href = savedSearchUrl;
        }

        // 2. Mark this job as viewed
        saveJobAsViewed(window.location.pathname);
    } else {
        // We are on a search or listing page:
        // 1. Mark already viewed jobs visually
        markViewedJobsOnPage();

        // 2. Restore scroll position if returning to the same URL (one-time restoration)
        const savedSearchUrl = sessionStorage.getItem('lastSearchUrl');
        const savedScrollY = sessionStorage.getItem('lastScrollPosition');
        const currentUrl = window.location.pathname + window.location.search;

        if (savedSearchUrl === currentUrl && savedScrollY) {
            const scrollPos = parseInt(savedScrollY, 10);
            sessionStorage.removeItem('lastSearchUrl');
            sessionStorage.removeItem('lastScrollPosition');

            if ('scrollRestoration' in history) {
                history.scrollRestoration = 'manual';
            }

            if (scrollPos > 0) {
                requestAnimationFrame(() => {
                    window.scrollTo({ top: scrollPos, behavior: 'instant' });
                });
            }
        }
    }

    // Global listener to capture the search page context and save viewed history when clicking any job card
    document.addEventListener('click', (e) => {
        const jobLink = e.target.closest('.job-card a, .job-card__title a');
        if (jobLink) {
            const href = jobLink.getAttribute('href');
            if (href && href.startsWith('/jobs/') && href !== '/jobs/' && href !== '/jobs') {
                // Save to viewed history
                saveJobAsViewed(href);

                // Only capture search context if we are NOT already on a detail page
                if (!isDetail) {
                    sessionStorage.setItem('lastSearchUrl', window.location.pathname + window.location.search);
                    sessionStorage.setItem('lastScrollPosition', window.scrollY.toString());
                }
            }
        }
    });

    // Dynamic Hero Text Rotator
    const rotator = document.querySelector('.hero__rotator');
    const rotatorWords = document.querySelectorAll('.hero__rotator-word');
    if (rotator && rotatorWords.length > 0) {
        let currentIndex = 0;

        // Helper function to measure and set rotator width to current word
        const setRotatorWidth = (index) => {
            const word = rotatorWords[index];
            word.classList.add('measure');
            const width = word.getBoundingClientRect().width;
            word.classList.remove('measure');
            rotator.style.width = `${width}px`;
        };

        // Initialize first word's width
        setRotatorWidth(currentIndex);

        // Run rotation loop after initial render settles
        setTimeout(() => {
            setInterval(() => {
                const activeWord = rotatorWords[currentIndex];
                activeWord.classList.remove('active');
                activeWord.classList.add('exit');

                currentIndex = (currentIndex + 1) % rotatorWords.length;

                const nextWord = rotatorWords[currentIndex];
                nextWord.classList.remove('exit');
                nextWord.classList.add('active');

                // Adjust width smoothly
                setRotatorWidth(currentIndex);

                setTimeout(() => {
                    activeWord.classList.remove('exit');
                }, 500);
            }, 3500);
        }, 4000);

        // Adjust on window resize and font load
        window.addEventListener('resize', () => setRotatorWidth(currentIndex));
        window.addEventListener('load', () => setRotatorWidth(currentIndex));
    }

    // Dynamic Dual Scroll Controller (Left Prev Button + Right Next Button)
    function setupDualScrollController(prevBtnId, nextBtnId, containerId, scrollAmount = 140) {
        const prevBtn = document.getElementById(prevBtnId);
        const nextBtn = document.getElementById(nextBtnId);
        const container = document.getElementById(containerId);
        if (!container) return;

        function updateButtons() {
            const maxScroll = Math.max(0, container.scrollWidth - container.clientWidth);
            const currentScroll = container.scrollLeft;

            // If no scrollable content exists at all, hide BOTH buttons completely
            if (maxScroll <= 3) {
                if (prevBtn) prevBtn.classList.remove('is-visible');
                if (nextBtn) nextBtn.classList.remove('is-visible');
                return;
            }

            // Left Previous Button: Only show if user has scrolled to the right (> 5px)
            if (prevBtn) {
                if (currentScroll > 5) {
                    prevBtn.classList.add('is-visible');
                } else {
                    prevBtn.classList.remove('is-visible');
                }
            }

            // Right Next Button: Only show if there is more content to scroll to the right
            if (nextBtn) {
                if (currentScroll < maxScroll - 5) {
                    nextBtn.classList.add('is-visible');
                } else {
                    nextBtn.classList.remove('is-visible');
                }
            }
        }

        if (prevBtn) {
            prevBtn.addEventListener('click', (e) => {
                e.preventDefault();
                container.scrollBy({ left: -scrollAmount, behavior: 'smooth' });
                setTimeout(updateButtons, 350);
            });
        }

        if (nextBtn) {
            nextBtn.addEventListener('click', (e) => {
                e.preventDefault();
                container.scrollBy({ left: scrollAmount, behavior: 'smooth' });
                setTimeout(updateButtons, 350);
            });
        }

        container.addEventListener('scroll', updateButtons, { passive: true });
        window.addEventListener('resize', updateButtons);
        document.addEventListener('DOMContentLoaded', updateButtons);
        window.addEventListener('load', updateButtons);
        setTimeout(updateButtons, 100);
        setTimeout(updateButtons, 400);
    }

    setupDualScrollController('popular-tags-prev', 'popular-tags-next', 'popular-tags-scroll', 140);
    setupDualScrollController('browse-tabs-prev', 'browse-tabs-next', 'browse-tabs-header', 140);

    // Horizontal Filter Bar Dropdowns Controller
    let listingLayout = document.querySelector('.listing-layout');

    // ==========================================
    // AJAX Filtering & Searching Controller
    // ==========================================
    let isLoadingFilters = false;
    let filterAbortController = null;

    const scrollToTopAnchor = () => {
        const targetEl = document.getElementById('jobs-top') || document.querySelector('.search-bar') || document.querySelector('.listing-layout');
        if (targetEl) {
            const headerHeight = document.getElementById('site-header')?.offsetHeight || 65;
            const elTop = targetEl.getBoundingClientRect().top + window.pageYOffset - headerHeight - 12;
            window.scrollTo({
                top: Math.max(0, elTop),
                behavior: 'auto'
            });
        } else {
            window.scrollTo(0, 0);
        }
    };

    const loadFilteredJobs = async (url, shouldScroll = false) => {
        if (isLoadingFilters) return; // Prevent concurrent requests
        isLoadingFilters = true;

        if (shouldScroll) {
            scrollToTopAnchor();
        }

        try {
            const response = await fetch(url);
            if (!response.ok) throw new Error('Network response was not ok');
            const html = await response.text();

            const parser = new DOMParser();
            const doc = parser.parseFromString(html, 'text/html');

            // 1. Replace Listing Header (Results counter)
            const newHeader = doc.querySelector('.listing-header');
            const oldHeader = document.querySelector('.listing-header');
            if (newHeader && oldHeader) {
                oldHeader.innerHTML = newHeader.innerHTML;
            }

            // 2. Replace Listing Main (Jobs list + Pagination block)
            const newMain = doc.querySelector('.listing-layout__main');
            const oldMain = document.querySelector('.listing-layout__main');
            if (newMain && oldMain) {
                oldMain.innerHTML = newMain.innerHTML;
            }

            // 3. Replace Sidebar Filters (Synchronizes checked states and item counts)
            const newSidebar = doc.querySelector('.listing-layout__sidebar');
            const oldSidebar = document.querySelector('.listing-layout__sidebar');
            if (newSidebar && oldSidebar) {
                oldSidebar.innerHTML = newSidebar.innerHTML;
            }

            // 4. Replace Horizontal Filter Bar (Synchronizes filter values)
            const newFilterBar = doc.querySelector('.filter-bar');
            const oldFilterBar = document.querySelector('.filter-bar');
            if (newFilterBar && oldFilterBar) {
                oldFilterBar.innerHTML = newFilterBar.innerHTML;
                if (newFilterBar.classList.contains('filter-bar--no-viewed')) {
                    oldFilterBar.classList.add('filter-bar--no-viewed');
                } else {
                    oldFilterBar.classList.remove('filter-bar--no-viewed');
                }
            }

            // 5. Update URL in address bar without reloading
            history.pushState(null, '', url);

            // 6. Re-bind event handlers to new elements
            bindListingFilters();

            // 6b. Re-initialize View Mode Controller & Observers for new DOM elements
            initViewModeController();
            initFooterBarObserver();

            // Update active filters badge
            updateActiveFiltersBadge();

            // 7. Ensure top position right after DOM replacement (instant show top result)
            if (shouldScroll) {
                scrollToTopAnchor();
            }

            // 8. Re-mark viewed cards
            if (typeof markViewedJobsOnPage === 'function') {
                markViewedJobsOnPage();
            }
            if (typeof updateViewedTabCount === 'function') {
                updateViewedTabCount();
            }

            // 9. Re-initialize & refresh ads in the newly injected layout
            if (typeof window.JobyUKRefreshAds === 'function') {
                window.JobyUKRefreshAds();
            }

        } catch (error) {
            console.error('Error loading filtered jobs:', error);
            window.location.href = url;
        } finally {
            isLoadingFilters = false;
        }
    };

    const updateActiveFiltersBadge = () => {
        const urlParams = new URLSearchParams(window.location.search);
        let count = 0;

        if (urlParams.get('easy_apply') === 'true') count++;
        if (urlParams.get('remote_type')) count++;
        if (urlParams.get('date_posted')) count++;
        if (urlParams.get('experience')) count++;
        if (urlParams.get('type')) count++;
        if (urlParams.get('job_type')) count++;

        const minSal = parseFloat(urlParams.get('min_salary'));
        if (!isNaN(minSal) && minSal > 0) count++;

        const maxSal = parseFloat(urlParams.get('max_salary'));
        if (!isNaN(maxSal) && maxSal > 0) count++;

        const badge = document.getElementById('filter-count-badge');
        const filterBtn = document.getElementById('search-bar-filter-toggle-btn');
        const clearBtn = document.getElementById('search-bar-clear-all-btn');
        const resetBtn = document.getElementById('sidebar-filters-reset-btn');

        if (badge) {
            badge.textContent = count;
            badge.style.display = count > 0 ? 'inline-flex' : 'none';
        }

        if (clearBtn) {
            clearBtn.style.display = count > 0 ? 'inline-flex' : 'none';
        }

        if (resetBtn) {
            if (count > 0) {
                resetBtn.classList.add('is-active');
                resetBtn.style.display = 'inline-flex';
            } else {
                resetBtn.classList.remove('is-active');
                resetBtn.style.display = 'none';
            }
        }

        if (filterBtn) {
            const currentLayout = document.querySelector('.listing-layout');
            const isSidebarOpen = currentLayout ? currentLayout.classList.contains('listing-layout--filters-open') : false;

            if (count > 0 || isSidebarOpen) {
                filterBtn.classList.add('search-bar__filter-btn--active');
            } else {
                filterBtn.classList.remove('search-bar__filter-btn--active');
            }
        }
    };

    function bindListingFilters() {
        // Abort all previous event listeners from last bind cycle
        if (filterAbortController) {
            filterAbortController.abort();
        }
        filterAbortController = new AbortController();
        const signal = filterAbortController.signal;

        const filterSearchForm = document.querySelector('.search-bar__form');
        const sidebarForm = document.querySelector('.filters form');
        const isListingPage = document.querySelector('.listing-layout__main') !== null;
        let isProgrammaticSubmit = false;

        // Clear / Reset All Active Filters Handler
        const handleClearAllFilters = () => {
            // Instant responsive feedback: hide badges and clear buttons immediately
            const badge = document.getElementById('filter-count-badge');
            const clearBtn = document.getElementById('search-bar-clear-all-btn');
            const resetBtn = document.getElementById('sidebar-filters-reset-btn');
            if (badge) badge.style.display = 'none';
            if (clearBtn) clearBtn.style.display = 'none';
            if (resetBtn) {
                resetBtn.classList.remove('is-active');
                resetBtn.style.display = 'none';
            }

            const currentUrl = new URL(window.location.href);
            const query = currentUrl.searchParams.get('q');
            const loc = currentUrl.searchParams.get('location');

            // Build clean target URL preserving search keywords if present
            const targetUrl = new URL(window.location.origin + '/jobs/');
            if (query) targetUrl.searchParams.set('q', query);
            if (loc) targetUrl.searchParams.set('location', loc);

            // Reset inputs and form values
            const sidebarFormEl = document.querySelector('.filters form');
            if (sidebarFormEl) {
                sidebarFormEl.reset();
            }

            const easyApplySwitch = document.getElementById('easy-apply-toggle-checkbox');
            if (easyApplySwitch) easyApplySwitch.checked = false;

            const minSalInput = document.getElementById('salary-min');
            const maxSalInput = document.getElementById('salary-max');
            if (minSalInput) minSalInput.value = '';
            if (maxSalInput) maxSalInput.value = '';

            const hiddenMinSalary = document.getElementById('hidden-min-salary');
            const hiddenMaxSalary = document.getElementById('hidden-max-salary');
            const hiddenContractType = document.getElementById('hidden-contract-type');
            const hiddenJobType = document.getElementById('hidden-job-type');
            if (hiddenMinSalary) hiddenMinSalary.value = '';
            if (hiddenMaxSalary) hiddenMaxSalary.value = '';
            if (hiddenContractType) hiddenContractType.value = '';
            if (hiddenJobType) hiddenJobType.value = '';

            // Also uncheck all filter radio inputs across DOM
            document.querySelectorAll('.filters input[type="radio"]').forEach(radio => {
                if (radio.value === '') {
                    radio.checked = true;
                } else {
                    radio.checked = false;
                }
            });

            loadFilteredJobs(targetUrl.toString(), true);
        };

        const searchBarClearBtn = document.getElementById('search-bar-clear-all-btn');
        if (searchBarClearBtn) {
            searchBarClearBtn.addEventListener('click', handleClearAllFilters, { signal });
        }

        const sidebarResetBtn = document.getElementById('sidebar-filters-reset-btn');
        if (sidebarResetBtn) {
            sidebarResetBtn.addEventListener('click', handleClearAllFilters, { signal });
        }

        // A. Sidebar Accordions
        const accordions = document.querySelectorAll(".filters__section.accordion");
        accordions.forEach(acc => {
            const header = acc.querySelector(".accordion__header");
            const content = acc.querySelector(".accordion__content");
            if (!header || !content) return;

            // Auto-expand initially if accordion contains any active/checked filter
            const checkInputs = acc.querySelectorAll("input[type='radio'], input[type='checkbox']");
            let hasActiveFilter = false;
            checkInputs.forEach(i => {
                if (i.checked && i.value !== "") {
                    hasActiveFilter = true;
                }
            });

            const salaryInputs = acc.querySelectorAll("input[type='number']");
            salaryInputs.forEach(i => {
                if (i.value && parseFloat(i.value) > 0) {
                    hasActiveFilter = true;
                }
            });

            if (hasActiveFilter) {
                acc.classList.add("active");
                header.setAttribute("aria-expanded", "true");
            }

            header.addEventListener("click", (e) => {
                e.preventDefault();
                e.stopPropagation();
                acc.classList.toggle("active");
                const isOpen = acc.classList.contains("active");
                header.setAttribute("aria-expanded", isOpen ? "true" : "false");
            }, { signal });
        });

        // B. Horizontal Filter Bar Dropdowns toggles
        const filterDropdowns = document.querySelectorAll('.filter-dropdown');
        filterDropdowns.forEach(dropdown => {
            const btn = dropdown.querySelector('.filter-dropdown__btn');
            if (btn) {
                btn.addEventListener('click', (e) => {
                    e.stopPropagation();
                    filterDropdowns.forEach(d => {
                        if (d !== dropdown) d.classList.remove('active');
                    });
                    dropdown.classList.toggle('active');
                }, { signal });
            }
        });

        // C. Dropdown menu click inside protection
        const menus = document.querySelectorAll('.filter-dropdown__menu');
        menus.forEach(menu => {
            menu.addEventListener('click', (e) => e.stopPropagation(), { signal });
        });

        // D. Contract Type select option listeners
        const contractTypeOpts = document.querySelectorAll('input[name="contract_type_opt"]');
        const hiddenContractType = document.getElementById('hidden-contract-type');
        contractTypeOpts.forEach(opt => {
            opt.addEventListener('change', () => {
                if (hiddenContractType && filterSearchForm) {
                    hiddenContractType.value = opt.value;
                    isProgrammaticSubmit = true;
                    filterSearchForm.requestSubmit();
                }
            }, { signal });
        });

        // E. Job Type select option listeners
        const jobTypeOpts = document.querySelectorAll('input[name="job_type_opt"]');
        const hiddenJobType = document.getElementById('hidden-job-type');
        jobTypeOpts.forEach(opt => {
            opt.addEventListener('change', () => {
                if (hiddenJobType && filterSearchForm) {
                    hiddenJobType.value = opt.value;
                    isProgrammaticSubmit = true;
                    filterSearchForm.requestSubmit();
                }
            }, { signal });
        });

        // F. Location selection select option listeners
        const locationOpts = document.querySelectorAll('input[name="location_opt"]');
        const mainLocationInput = document.getElementById('location-input');
        locationOpts.forEach(opt => {
            opt.addEventListener('change', () => {
                if (mainLocationInput && filterSearchForm) {
                    mainLocationInput.value = opt.value;
                    isProgrammaticSubmit = true;
                    filterSearchForm.requestSubmit();
                }
            }, { signal });
        });

        // G. Yearly Salary Apply button listener
        const applySalaryBtn = document.getElementById('apply-salary-btn');
        const salaryMinInput = document.getElementById('salary-min');
        const salaryMaxInput = document.getElementById('salary-max');
        const hiddenMinSalary = document.getElementById('hidden-min-salary');
        const hiddenMaxSalary = document.getElementById('hidden-max-salary');
        if (applySalaryBtn && filterSearchForm) {
            applySalaryBtn.addEventListener('click', () => {
                if (hiddenMinSalary && salaryMinInput) hiddenMinSalary.value = salaryMinInput.value;
                if (hiddenMaxSalary && salaryMaxInput) hiddenMaxSalary.value = salaryMaxInput.value;
                isProgrammaticSubmit = true;
                filterSearchForm.requestSubmit();
            }, { signal });
        }

        // H. Easily Apply Toggle Checkbox handler
        const easyApplyCheckbox = document.getElementById('easy-apply-toggle-checkbox');
        if (easyApplyCheckbox) {
            easyApplyCheckbox.addEventListener('change', function () {
                if (sidebarForm) {
                    const applyEasyRadio = document.getElementById('apply-easy');
                    const applyAllRadio = document.getElementById('apply-all');
                    if (this.checked) {
                        if (applyEasyRadio) applyEasyRadio.checked = true;
                    } else {
                        if (applyAllRadio) applyAllRadio.checked = true;
                    }
                    sidebarForm.requestSubmit();
                }
            }, { signal });
        }

        // I. Intercept pagination clicks for AJAX
        const paginationLinks = document.querySelectorAll(".pagination a");
        paginationLinks.forEach(link => {
            link.addEventListener("click", (e) => {
                e.preventDefault();
                scrollToTopAnchor();
                loadFilteredJobs(link.href, true);
            }, { signal });
        });

        // J. Intercept sidebar form submit events
        if (sidebarForm && isListingPage) {
            sidebarForm.addEventListener('submit', (e) => {
                e.preventDefault();
                const formData = new FormData(sidebarForm);
                const params = new URLSearchParams(formData);
                for (const [key, value] of [...params.entries()]) {
                    if (value === "") params.delete(key);
                }
                const url = `${sidebarForm.action}?${params.toString()}`;
                loadFilteredJobs(url, false);
            }, { signal });
        }

        // K. Intercept main search bar form submit events (only on listing page)
        if (filterSearchForm && isListingPage) {
            filterSearchForm.addEventListener('submit', (e) => {
                e.preventDefault();
                const formData = new FormData(filterSearchForm);
                const params = new URLSearchParams(formData);
                for (const [key, value] of [...params.entries()]) {
                    if (value === "") params.delete(key);
                }
                const url = `${filterSearchForm.action}?${params.toString()}`;

                const shouldScroll = !isProgrammaticSubmit;
                isProgrammaticSubmit = false;

                loadFilteredJobs(url, shouldScroll);
            }, { signal });
        }

        // L. All Filters Sidebar toggle and layout state sync
        const allFiltersBtn = document.getElementById('all-filters-toggle-btn');
        const searchBarFilterBtn = document.getElementById('search-bar-filter-toggle-btn');
        const closeDrawerBtn = document.getElementById('filters-close-drawer-btn');
        const filtersBackdrop = document.getElementById('filters-backdrop');
        listingLayout = document.querySelector('.listing-layout');

        if (listingLayout && (allFiltersBtn || searchBarFilterBtn || closeDrawerBtn)) {
            const isMobile = window.innerWidth < 1024;
            const filtersOpen = isMobile ? false : (localStorage.getItem('filtersOpen') === 'true');

            const setFiltersState = (open) => {
                const currentLayout = document.querySelector('.listing-layout');
                const currentAllBtn = document.getElementById('all-filters-toggle-btn');
                const currentSearchBtn = document.getElementById('search-bar-filter-toggle-btn');

                if (currentLayout) {
                    if (open) {
                        currentLayout.classList.add('listing-layout--filters-open');
                        if (currentAllBtn) currentAllBtn.classList.add('filter-bar__all-btn--active');
                        if (currentSearchBtn) currentSearchBtn.classList.add('search-bar__filter-btn--active');
                        if (window.innerWidth < 1024) {
                            document.body.style.overflow = 'hidden';
                        } else {
                            localStorage.setItem('filtersOpen', 'true');
                        }
                    } else {
                        currentLayout.classList.remove('listing-layout--filters-open');
                        if (currentAllBtn) currentAllBtn.classList.remove('filter-bar__all-btn--active');
                        document.body.style.overflow = '';

                        // Preserve active button style if url has active filters
                        const urlParams = new URLSearchParams(window.location.search);
                        let count = 0;
                        if (urlParams.get('easy_apply') === 'true') count++;
                        if (urlParams.get('remote_type')) count++;
                        if (urlParams.get('date_posted')) count++;
                        if (urlParams.get('experience')) count++;
                        if (urlParams.get('type')) count++;
                        if (urlParams.get('job_type')) count++;
                        if (urlParams.get('location')) count++;
                        const minSal = parseFloat(urlParams.get('min_salary'));
                        if (!isNaN(minSal) && minSal > 0) count++;
                        const maxSal = parseFloat(urlParams.get('max_salary'));
                        if (!isNaN(maxSal) && maxSal > 0) count++;

                        if (currentSearchBtn) {
                            if (count > 0) {
                                currentSearchBtn.classList.add('search-bar__filter-btn--active');
                            } else {
                                currentSearchBtn.classList.remove('search-bar__filter-btn--active');
                            }
                        }
                        if (window.innerWidth >= 1024) {
                            localStorage.setItem('filtersOpen', 'false');
                        }
                    }
                }
            };

            setFiltersState(filtersOpen);

            if (allFiltersBtn) {
                allFiltersBtn.addEventListener('click', () => {
                    const currentLayout = document.querySelector('.listing-layout');
                    const isOpen = currentLayout ? currentLayout.classList.contains('listing-layout--filters-open') : false;
                    setFiltersState(!isOpen);
                }, { signal });
            }

            if (searchBarFilterBtn) {
                searchBarFilterBtn.addEventListener('click', () => {
                    const currentLayout = document.querySelector('.listing-layout');
                    const isOpen = currentLayout ? currentLayout.classList.contains('listing-layout--filters-open') : false;
                    setFiltersState(!isOpen);
                }, { signal });
            }

            if (closeDrawerBtn) {
                closeDrawerBtn.addEventListener('click', () => {
                    setFiltersState(false);
                }, { signal });
            }

            if (filtersBackdrop) {
                filtersBackdrop.addEventListener('click', () => {
                    setFiltersState(false);
                }, { signal });
            }
        }

        // M. Viewed Jobs Tab toggles (inside filter-bar, replaced by AJAX)
        const tabAllJobs = document.getElementById('tab-all-jobs');
        const tabViewedJobs = document.getElementById('tab-viewed-jobs');
        const pagination = document.querySelector('.pagination');
        const viewedPane = document.getElementById('viewed-jobs-history-pane');

        if (tabAllJobs && tabViewedJobs && viewedPane) {
            tabAllJobs.addEventListener('click', () => {
                tabViewedJobs.classList.remove('view-segment-btn--active');
                tabAllJobs.classList.add('view-segment-btn--active');
                viewedPane.style.display = 'none';
                const currentListingLayout = document.querySelector('.listing-layout');
                if (currentListingLayout) currentListingLayout.style.display = '';
                const currentPagination = document.querySelector('.pagination');
                if (currentPagination) currentPagination.style.display = '';
                const paginationModeWrap = document.getElementById('pagination-mode-wrap');
                if (paginationModeWrap) paginationModeWrap.style.display = '';
                const infiniteModeWrap = document.getElementById('infinite-mode-wrap');
                if (infiniteModeWrap && infiniteModeWrap.getAttribute('data-active') === 'true') infiniteModeWrap.style.display = '';
                const stickyModeBar = document.getElementById('sticky-view-mode-bar');
                if (stickyModeBar) stickyModeBar.style.display = '';
            }, { signal });

            tabViewedJobs.addEventListener('click', () => {
                tabAllJobs.classList.remove('view-segment-btn--active');
                tabViewedJobs.classList.add('view-segment-btn--active');
                const currentListingLayout = document.querySelector('.listing-layout');
                if (currentListingLayout) currentListingLayout.style.display = 'none';
                const currentPagination = document.querySelector('.pagination');
                if (currentPagination) currentPagination.style.display = 'none';
                const paginationModeWrap = document.getElementById('pagination-mode-wrap');
                if (paginationModeWrap) paginationModeWrap.style.display = 'none';
                const infiniteModeWrap = document.getElementById('infinite-mode-wrap');
                if (infiniteModeWrap) infiniteModeWrap.style.display = 'none';
                const stickyModeBar = document.getElementById('sticky-view-mode-bar');
                if (stickyModeBar) stickyModeBar.style.display = 'none';
                renderViewedJobsHistory();
                viewedPane.style.display = 'block';
            }, { signal });
        }
    }

    // Initialize list filters and AJAX listeners on page load
    bindListingFilters();
    updateActiveFiltersBadge();

    // Setup Clear History Modal toggles
    const clearBtn = document.getElementById('vh-clear-btn');
    const clearModal = document.getElementById('clear-history-modal');
    const modalCancel = document.getElementById('confirm-modal-cancel');
    const modalConfirm = document.getElementById('confirm-modal-yes');

    if (clearBtn && clearModal && modalCancel && modalConfirm) {
        clearBtn.addEventListener('click', () => {
            clearModal.classList.add('open');
        });

        const closeModal = () => {
            clearModal.classList.remove('open');
        };

        modalCancel.addEventListener('click', closeModal);

        // Close on overlay click
        clearModal.addEventListener('click', (e) => {
            if (e.target === clearModal) {
                closeModal();
            }
        });

        modalConfirm.addEventListener('click', () => {
            localStorage.removeItem('viewedJobsDetails');
            localStorage.removeItem('viewedJobs');
            if (typeof renderViewedJobsHistory === 'function') {
                renderViewedJobsHistory();
            }
            if (typeof updateViewedTabCount === 'function') {
                updateViewedTabCount();
            }

            // Clean viewed cards border highlights from DOM
            document.querySelectorAll('.job-card--viewed').forEach(c => {
                c.classList.remove('job-card--viewed');
                const pill = c.querySelector('.job-pill--viewed');
                if (pill) pill.remove();
            });

            closeModal();
        });
    }

    // Close filter bar dropdowns globally on outside click
    document.addEventListener('click', () => {
        const filterDropdowns = document.querySelectorAll('.filter-dropdown');
        filterDropdowns.forEach(d => d.classList.remove('active'));
    });

    // Support browser Back/Forward navigation with AJAX load
    window.addEventListener('popstate', () => {
        const isListingPage = document.querySelector('.listing-layout__main') !== null;
        if (isListingPage) {
            loadFilteredJobs(window.location.href);
        }
        if (typeof window.JobyUKRefreshAds === 'function') {
            window.JobyUKRefreshAds();
        }
    });

    // ==========================================
    // Recently Viewed Positions Controller
    // ==========================================

    // Capture job details on card click
    const saveJobAsViewedDetails = (card) => {
        const titleLink = card.querySelector('.job-card__title a');
        if (!titleLink) return;
        const href = titleLink.getAttribute('href');
        const title = titleLink.textContent.trim();

        const employerEl = card.querySelector('.job-card__submeta-item');
        const employer = employerEl ? employerEl.textContent.trim() : '';

        const metaItems = card.querySelectorAll('.job-card__submeta-item');
        const location = metaItems.length > 2 ? metaItems[2].textContent.trim() : '';

        const salaryEl = Array.from(metaItems).find(el => el.style.color && el.style.color.includes('color-primary'));
        const salary = salaryEl ? salaryEl.textContent.trim() : '';

        const logoEl = card.querySelector('.job-card__logo');
        const logoHtml = logoEl ? logoEl.innerHTML : '';

        const parts = href.split('/').filter(Boolean);
        const slug = parts[parts.length - 1];

        if (slug) {
            let viewed = [];
            try {
                viewed = JSON.parse(localStorage.getItem('viewedJobsDetails')) || [];
            } catch (e) {
                viewed = [];
            }
            if (!Array.isArray(viewed)) viewed = [];

            const index = viewed.findIndex(item => item.slug === slug);
            if (index !== -1) {
                viewed.splice(index, 1);
            }
            viewed.unshift({ slug, href, title, employer, location, salary, logoHtml, timestamp: Date.now() });
            if (viewed.length > 50) viewed.pop();
            localStorage.setItem('viewedJobsDetails', JSON.stringify(viewed));

            // Also keep standard viewedJobs slugs synchronized
            let slugs = [];
            try {
                slugs = JSON.parse(localStorage.getItem('viewedJobs')) || [];
            } catch (e) {
                slugs = [];
            }
            if (!slugs.includes(slug)) {
                slugs.push(slug);
                localStorage.setItem('viewedJobs', JSON.stringify(slugs));
            }
        }
    };

    // Capture job details on direct page view
    const saveCurrentJobDetail = () => {
        if (!window.location.pathname.startsWith('/jobs/') || window.location.pathname === '/jobs/') return;

        const titleEl = document.querySelector('.job-detail-hero__title');
        if (!titleEl) return;

        const title = titleEl.textContent.trim();
        const employerEl = document.querySelector('.employer-name-link') || document.querySelector('.employer-name-text');
        const employer = employerEl ? employerEl.textContent.trim() : '';

        const metaItem = document.querySelector('.job-detail-hero__meta-item');
        const location = metaItem ? metaItem.textContent.trim() : '';

        const salaryEl = document.querySelector('.job-detail-hero__meta-item:nth-of-type(3)');
        const salary = salaryEl ? salaryEl.textContent.trim() : '';

        const logoEl = document.querySelector('.job-detail-hero__logo');
        const logoHtml = logoEl ? logoEl.innerHTML : '';

        const href = window.location.pathname;
        const parts = href.split('/').filter(Boolean);
        const slug = parts[parts.length - 1];

        if (slug) {
            let viewed = [];
            try {
                viewed = JSON.parse(localStorage.getItem('viewedJobsDetails')) || [];
            } catch (e) {
                viewed = [];
            }
            if (!Array.isArray(viewed)) viewed = [];

            const index = viewed.findIndex(item => item.slug === slug);
            if (index !== -1) {
                viewed.splice(index, 1);
            }
            viewed.unshift({ slug, href, title, employer, location, salary, logoHtml, timestamp: Date.now() });
            if (viewed.length > 50) viewed.pop();
            localStorage.setItem('viewedJobsDetails', JSON.stringify(viewed));

            let slugs = [];
            try {
                slugs = JSON.parse(localStorage.getItem('viewedJobs')) || [];
            } catch (e) {
                slugs = [];
            }
            if (!slugs.includes(slug)) {
                slugs.push(slug);
                localStorage.setItem('viewedJobs', JSON.stringify(slugs));
            }
        }
    };

    // Render viewed jobs history list inside the tab pane
    const renderViewedJobsHistory = () => {
        const listEl = document.getElementById('viewed-history-list');
        if (!listEl) return;

        let viewed = [];
        try {
            viewed = JSON.parse(localStorage.getItem('viewedJobsDetails')) || [];
        } catch (e) {
            viewed = [];
        }

        // Update tab count badge
        updateViewedTabCount();

        if (viewed.length === 0) {
            listEl.innerHTML = `
                <div class="viewed-history-empty">
                    <svg class="viewed-history-empty__illustration" width="64" height="64" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.2">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                        <path stroke-linecap="round" stroke-linejoin="round" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                    </svg>
                    <h4 class="viewed-history-empty__title">No Visited Jobs Yet</h4>
                    <p class="viewed-history-empty__text">Start exploring positions to build your local timezone search history.</p>
                </div>
            `;
            return;
        }

        const fragment = document.createDocumentFragment();
        viewed.forEach(item => {
            const dateObj = new Date(item.timestamp);
            const formattedDate = dateObj.toLocaleDateString(undefined, {
                weekday: 'short',
                month: 'short',
                day: 'numeric',
                year: 'numeric'
            });
            const formattedTime = dateObj.toLocaleTimeString(undefined, {
                hour: '2-digit',
                minute: '2-digit'
            });
            const localTimeString = `${formattedDate}, ${formattedTime}`;

            let logoMarkup = item.logoHtml;
            if (!logoMarkup || logoMarkup.trim() === '') {
                const initial = item.employer ? item.employer.charAt(0).toUpperCase() : 'J';
                logoMarkup = `<div style="font-weight: 700; color: #0A66C2; font-size: 18px;">${initial}</div>`;
            }

            const article = document.createElement('article');
            article.className = 'job-card job-card--viewed animate-fade-in';
            article.innerHTML = `
                <div class="job-card__main">
                    <div class="job-card__logo" aria-hidden="true">
                        ${logoMarkup}
                    </div>
                    <div class="job-card__content">
                        <h3 class="job-card__title">
                            <a href="${item.href}">${item.title}</a>
                        </h3>
                        <div class="job-card__submeta">
                            <span class="job-card__submeta-item" style="font-weight: 600; color: #1e293b;">${item.employer}</span>
                            <span class="job-card__submeta-dot">•</span>
                            <span class="job-card__submeta-item">${item.location}</span>
                            ${item.salary ? `
                                <span class="job-card__submeta-dot">•</span>
                                <span class="job-card__submeta-item" style="color: var(--color-primary, #0a66c2); font-weight: 600;">${item.salary}</span>
                            ` : ''}
                        </div>
                        <div class="job-card__pills">
                            <span class="job-pill job-pill--viewed">
                                <svg width="10" height="10" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5" style="display:inline-block;vertical-align:middle;margin-right:3px;">
                                    <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                                </svg>
                                Viewed
                            </span>
                        </div>
                    </div>
                </div>
                <div class="job-card__right">
                    <span class="job-card__time">${localTimeString}</span>
                </div>
            `;
            fragment.appendChild(article);
        });

        // Use replaceChildren to prevent scroll jumping caused by innerHTML='' height collapse
        listEl.replaceChildren(fragment);
    };

    // Update Viewed Jobs Tab Badge count and visibility of view switcher
    const updateViewedTabCount = () => {
        const tabCount = document.getElementById('viewed-tab-count');
        const tabsWrapper = document.getElementById('view-tabs-wrapper');
        const filterBar = document.querySelector('.filter-bar');

        let viewed = [];
        try {
            viewed = JSON.parse(localStorage.getItem('viewedJobsDetails')) || [];
        } catch (e) {
            viewed = [];
        }

        if (tabCount) {
            tabCount.textContent = viewed.length;
            tabCount.style.display = viewed.length > 0 ? 'inline-block' : 'none';
        }

        if (tabsWrapper) {
            tabsWrapper.style.display = viewed.length > 0 ? 'flex' : 'none';
        }

        if (filterBar) {
            if (viewed.length === 0) {
                filterBar.classList.add('filter-bar--no-viewed');
            } else {
                filterBar.classList.remove('filter-bar--no-viewed');
            }
        }

        // If history is cleared/empty, automatically switch back to the "All Openings" view
        if (viewed.length === 0) {
            const tabAllJobs = document.getElementById('tab-all-jobs');
            const tabViewedJobs = document.getElementById('tab-viewed-jobs');
            const pagination = document.querySelector('.pagination');
            const viewedPane = document.getElementById('viewed-jobs-history-pane');
            const listingLayout = document.querySelector('.listing-layout');

            if (tabAllJobs) tabAllJobs.classList.add('view-segment-btn--active');
            if (tabViewedJobs) tabViewedJobs.classList.remove('view-segment-btn--active');
            if (listingLayout) listingLayout.style.display = '';
            if (pagination) pagination.style.display = '';
            if (viewedPane) viewedPane.style.display = 'none';
        }
    };

    // Click handler to register viewed job cards on click
    document.addEventListener('click', (e) => {
        const card = e.target.closest('.job-card');
        if (card) {
            // Only update history metadata if clicking on a regular listing job (avoid re-rendering while browsing viewed history)
            if (!card.closest('#viewed-jobs-history-pane')) {
                saveJobAsViewedDetails(card);
                setTimeout(() => {
                    updateViewedTabCount();
                }, 50);
            }
        }
    });

    // Job Detail Page Interaction (Strict CSP Compliant)
    const stickyBar = document.getElementById('stickyDetailBar');
    if (stickyBar) {
        window.addEventListener('scroll', () => {
            if (window.scrollY > 200) {
                stickyBar.classList.add('visible');
            } else {
                stickyBar.classList.remove('visible');
            }
        }, { passive: true });
    }

    // Company Detail Page Interaction (Sticky Header on Scroll)
    const stickyCompanyBar = document.getElementById('stickyCompanyBar');
    if (stickyCompanyBar) {
        window.addEventListener('scroll', () => {
            if (window.scrollY > 160) {
                stickyCompanyBar.classList.add('visible');
            } else {
                stickyCompanyBar.classList.remove('visible');
            }
        }, { passive: true });
    }

    // Salary Guide Detail Page Interaction (Sticky Header on Scroll)
    const stickySalaryBar = document.getElementById('stickySalaryBar');
    if (stickySalaryBar) {
        window.addEventListener('scroll', () => {
            if (window.scrollY > 160) {
                stickySalaryBar.classList.add('visible');
            } else {
                stickySalaryBar.classList.remove('visible');
            }
        }, { passive: true });
    }

    // Clean and Format Job Description Body (remove clunky emojis & enhance typography)
    function formatJobDescriptionBody() {
        const body = document.querySelector('.job-detail-body');
        if (!body) return;

        const emojiRegex = /[\u{1F300}-\u{1F9FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}\u{1F1E6}-\u{1F1FF}\u{1FA70}-\u{1FAFF}]/gu;
        const walker = document.createTreeWalker(body, NodeFilter.SHOW_TEXT, null, false);
        let node;
        const textNodes = [];
        while (node = walker.nextNode()) {
            if (emojiRegex.test(node.nodeValue)) {
                textNodes.push(node);
            }
        }
        textNodes.forEach(t => {
            t.nodeValue = t.nodeValue.replace(emojiRegex, '').replace(/\s+/g, ' ');
        });
    }
    formatJobDescriptionBody();

    // Save/Bookmark Main Toggles (updates all main save buttons on detail page)
    const saveToggles = document.querySelectorAll('.btn-save-toggle');
    saveToggles.forEach(btn => {
        btn.addEventListener('click', () => {
            const isActive = btn.classList.toggle('active');
            saveToggles.forEach(otherBtn => {
                otherBtn.classList.toggle('active', isActive);
                const icon = otherBtn.querySelector('.bookmark-icon');
                if (icon) {
                    if (isActive) {
                        icon.setAttribute('fill', 'currentColor');
                    } else {
                        icon.setAttribute('fill', 'none');
                    }
                }
            });
        });
    });

    // Similar Jobs Save Toggles
    const simBookmarks = document.querySelectorAll('.ref-sim-card__bookmark');
    simBookmarks.forEach(btn => {
        btn.addEventListener('click', (e) => {
            e.preventDefault();
            const isActive = btn.classList.toggle('active');
            const icon = btn.querySelector('svg');
            if (icon) {
                if (isActive) {
                    icon.setAttribute('fill', 'currentColor');
                } else {
                    icon.setAttribute('fill', 'none');
                }
            }
        });
    });

    // Share Job Link Action (Modern floating dropdown context menu)
    const shareButtons = document.querySelectorAll('.btn-share-job');

    // Create the global floating share menu container if it doesn't exist
    let shareMenu = document.getElementById('global-share-menu');
    if (!shareMenu && shareButtons.length > 0) {
        shareMenu = document.createElement('div');
        shareMenu.id = 'global-share-menu';
        shareMenu.className = 'floating-share-menu';
        document.body.appendChild(shareMenu);
    }

    const closeShareMenu = () => {
        if (shareMenu) {
            shareMenu.classList.remove('open');
        }
        shareButtons.forEach(b => b.classList.remove('active'));
    };

    shareButtons.forEach(btn => {
        btn.addEventListener('click', (e) => {
            e.stopPropagation(); // Prevent immediate closing click event

            const wasActive = btn.classList.contains('active');
            closeShareMenu();

            if (!wasActive) {
                btn.classList.add('active');

                const url = window.location.href;
                const title = document.title || "Job Opportunity";

                // Build inner HTML with brand-colored social sharing classes and filled SVGs
                shareMenu.innerHTML = `
                <a href="https://www.google.com/preferences/source?q=jobyuk.com" target="_blank" rel="noopener noreferrer" class="floating-share-menu__item floating-share-menu__item--google">
                    <svg class="google-icon-svg" viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
                        <path fill="#4285F4" d="M23.745 12.27c0-.7-.06-1.4-.19-2.07H12v4.51h6.6c-.29 1.52-1.14 2.82-2.4 3.68v3.05h3.88c2.27-2.09 3.665-5.17 3.665-9.17z"/>
                        <path fill="#34A853" d="M12 24c3.24 0 5.95-1.08 7.93-2.91l-3.88-3.05c-1.08.72-2.45 1.16-4.05 1.16-3.12 0-5.77-2.1-6.72-4.93H1.25v3.15C3.26 21.36 7.34 24 12 24z"/>
                        <path fill="#FBBC05" d="M5.28 14.27c-.25-.72-.38-1.49-.38-2.27s.13-1.55.38-2.27V6.58H1.25C.45 8.18 0 9.98 0 12s.45 3.82 1.25 5.42l4.03-3.15z"/>
                        <path fill="#EA4335" d="M12 4.75c1.77 0 3.35.61 4.6 1.8l3.42-3.42C17.95 1.19 15.24 0 12 0 7.34 0 3.26 2.64 1.25 6.58l4.03 3.15c.95-2.83 3.6-4.98 6.72-4.98z"/>
                    </svg>
                    <span>Follow on Google</span>
                </a>
                <a href="https://www.linkedin.com/sharing/share-offsite/?url=${encodeURIComponent(url)}" target="_blank" rel="noopener noreferrer" class="floating-share-menu__item floating-share-menu__item--linkedin">
                    <svg width="15" height="15" viewBox="0 0 24 24" fill="currentColor"><path d="M19 0h-14c-2.761 0-5 2.239-5 5v14c0 2.761 2.239 5 5 5h14c2.762 0 5-2.239 5-5v-14c0-2.761-2.238-5-5-5zm-11 19h-3v-11h3v11zm-1.5-12.268c-.966 0-1.75-.79-1.75-1.764s.784-1.764 1.75-1.764 1.75.79 1.75 1.764-.783 1.764-1.75 1.764zm13.5 12.268h-3v-5.604c0-3.368-4-3.113-4 0v5.604h-3v-11h3v1.765c1.396-2.586 7-2.777 7 2.476v6.759z"/></svg>
                    <span>LinkedIn</span>
                </a>
                <a href="https://twitter.com/intent/tweet?url=${encodeURIComponent(url)}&text=${encodeURIComponent(title)}" target="_blank" rel="noopener noreferrer" class="floating-share-menu__item floating-share-menu__item--x">
                    <svg width="15" height="15" viewBox="0 0 24 24" fill="currentColor"><path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z"/></svg>
                    <span>X (Twitter)</span>
                </a>
                <a href="https://api.whatsapp.com/send?text=${encodeURIComponent(title + " " + url)}" target="_blank" rel="noopener noreferrer" class="floating-share-menu__item floating-share-menu__item--whatsapp">
                    <svg width="15" height="15" viewBox="0 0 24 24" fill="currentColor"><path d="M17.472 14.382c-.297-.149-1.758-.867-2.03-.967-.273-.099-.471-.148-.67.15-.197.297-.767.966-.94 1.164-.173.199-.347.223-.644.075-.297-.15-1.255-.463-2.39-1.475-.883-.788-1.48-1.761-1.653-2.059-.173-.297-.018-.458.13-.606.134-.133.298-.347.446-.52.149-.174.198-.298.298-.497.099-.198.05-.371-.025-.52-.075-.149-.669-1.612-.916-2.207-.242-.579-.487-.5-.669-.51-.173-.008-.371-.01-.57-.01-.198 0-.52.074-.792.372-.272.297-1.04 1.016-1.04 2.479 0 1.462 1.065 2.875 1.213 3.074.149.198 2.096 3.2 5.077 4.487.709.306 1.262.489 1.694.625.712.227 1.36.195 1.871.118.571-.085 1.758-.719 2.006-1.413.248-.694.248-1.289.173-1.413-.074-.124-.272-.198-.57-.347m-5.421 7.403h-.004a9.87 9.87 0 01-5.031-1.378l-.361-.214-3.741.982.998-3.648-.235-.374a9.86 9.86 0 01-1.51-5.26c.001-5.45 4.436-9.884 9.888-9.884 2.64 0 5.122 1.03 6.988 2.898a9.825 9.825 0 012.893 6.994c-.003 5.45-4.437 9.884-9.885 9.884m8.413-18.297A11.815 11.815 0 0012.05 0C5.495 0 .16 5.335.157 11.892c0 2.096.547 4.142 1.588 5.945L0 24l6.335-1.662c1.746.953 3.71 1.458 5.704 1.459h.005c6.554 0 11.89-5.335 11.893-11.893a11.821 11.821 0 00-3.48-8.413z"/>
                    </svg>
                    <span>WhatsApp</span>
                </a>
                <a href="https://www.facebook.com/sharer/sharer.php?u=${encodeURIComponent(url)}" target="_blank" rel="noopener noreferrer" class="floating-share-menu__item floating-share-menu__item--facebook">
                    <svg width="15" height="15" viewBox="0 0 24 24" fill="currentColor"><path d="M24 12.073c0-6.627-5.373-12-12-12s-12 5.373-12 12c0 5.99 4.388 10.954 10.125 11.854v-8.385H7.078v-3.47h3.047V9.43c0-3.007 1.792-4.669 4.533-4.669 1.312 0 2.686.235 2.686.235v2.953H15.83c-1.491 0-1.956.925-1.956 1.874v2.25h3.328l-.532 3.47h-2.796v8.385C19.612 23.027 24 18.062 24 12.073z"/></svg>
                    <span>Facebook</span>
                </a>
                <a href="mailto:?subject=${encodeURIComponent(title)}&body=${encodeURIComponent(url)}" class="floating-share-menu__item floating-share-menu__item--email">
                    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"></path><polyline points="22,6 12,13 2,6"></polyline></svg>
                    <span>Email</span>
                </a>
                <button type="button" class="floating-share-menu__item floating-share-menu__item--copy btn-copy-link-action">
                    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
                    <span>Copy Link</span>
                </button>
            `;

                // Setup Copy Link sub-button action inside menu
                const copyBtn = shareMenu.querySelector('.btn-copy-link-action');
                if (copyBtn) {
                    copyBtn.addEventListener('click', (ev) => {
                        ev.stopPropagation();
                        navigator.clipboard.writeText(url).then(() => {
                            const labelSpan = copyBtn.querySelector('span');
                            if (labelSpan) {
                                labelSpan.textContent = 'Copied!';
                                setTimeout(() => {
                                    labelSpan.textContent = 'Copy Link';
                                    closeShareMenu();
                                }, 1200);
                            }
                        }).catch(err => {
                            console.error('Could not copy link:', err);
                        });
                    });
                }

                // Position shareMenu below the clicked button
                const rect = btn.getBoundingClientRect();
                const scrollTop = window.scrollY || document.documentElement.scrollTop;
                const scrollLeft = window.scrollX || document.documentElement.scrollLeft;

                // Set positions
                shareMenu.style.top = `${rect.bottom + scrollTop + 6}px`;

                // Adjust left to prevent overflowing viewport right border
                const menuWidth = 180;
                let leftPos = rect.left + scrollLeft + (rect.width / 2) - (menuWidth / 2);
                if (leftPos + menuWidth > window.innerWidth) {
                    leftPos = window.innerWidth - menuWidth - 16;
                }
                if (leftPos < 16) {
                    leftPos = 16;
                }
                shareMenu.style.left = `${leftPos}px`;

                // Show menu
                shareMenu.classList.add('open');
            }
        });
    });

    // Mobile Bottom Sheets for Job Detail (Specs & Share)
    const specsSheet = document.getElementById('job-specs-sheet');
    const btnOpenSpecs = document.getElementById('btn-open-specs-sheet');
    const btnCloseSpecs = document.getElementById('specs-sheet-close');
    const backdropSpecs = document.getElementById('specs-sheet-backdrop');

    const openBottomSheet = (sheet) => {
        if (!sheet) return;
        sheet.classList.add('mobile-bottom-sheet--open');
        document.body.style.overflow = 'hidden';
    };

    const closeBottomSheet = (sheet) => {
        if (!sheet) return;
        sheet.classList.remove('mobile-bottom-sheet--open');
        document.body.style.overflow = '';
    };

    if (btnOpenSpecs && specsSheet) {
        btnOpenSpecs.addEventListener('click', () => openBottomSheet(specsSheet));
    }
    if (btnCloseSpecs && specsSheet) {
        btnCloseSpecs.addEventListener('click', () => closeBottomSheet(specsSheet));
    }
    if (backdropSpecs && specsSheet) {
        backdropSpecs.addEventListener('click', () => closeBottomSheet(specsSheet));
    }

    // Swipe-down-to-close gesture support for all bottom sheets
    document.querySelectorAll('.mobile-bottom-sheet').forEach(sheet => {
        let startY = 0;
        let currentY = 0;
        const card = sheet.querySelector('.mobile-bottom-sheet__card');
        const handle = sheet.querySelector('.mobile-bottom-sheet__handle-bar') || sheet.querySelector('.mobile-bottom-sheet__header');

        if (card && handle) {
            handle.addEventListener('touchstart', (e) => {
                startY = e.touches[0].clientY;
                currentY = startY;
            }, { passive: true });

            handle.addEventListener('touchmove', (e) => {
                currentY = e.touches[0].clientY;
                const diff = currentY - startY;
                if (diff > 0) {
                    card.style.transform = `translateY(${diff}px)`;
                    card.style.transition = 'none';
                }
            }, { passive: true });

            handle.addEventListener('touchend', () => {
                const diff = currentY - startY;
                card.style.transition = '';
                card.style.transform = '';
                if (diff > 70) {
                    closeBottomSheet(sheet);
                }
            }, { passive: true });
        }
    });

    // Share Bottom Sheet
    const shareSheet = document.getElementById('job-share-sheet');
    const btnOpenShareSheet = document.getElementById('btn-open-share-sheet');
    const btnCloseShareSheet = document.getElementById('share-sheet-close');
    const backdropShareSheet = document.getElementById('share-sheet-backdrop');
    const shareLinkInput = document.getElementById('share-link-input');
    const btnCopyShareLink = document.getElementById('btn-copy-share-link');
    const copyBtnText = document.getElementById('copy-btn-text');

    if (shareSheet) {
        const pageUrl = window.location.href;
        const pageTitle = document.title || "Job Opportunity on JobyUK.com";

        // Setup share URLs
        const whatsappBtn = document.getElementById('share-btn-whatsapp');
        if (whatsappBtn) whatsappBtn.href = `https://api.whatsapp.com/send?text=${encodeURIComponent(pageTitle + " - " + pageUrl)}`;

        const linkedinBtn = document.getElementById('share-btn-linkedin');
        if (linkedinBtn) linkedinBtn.href = `https://www.linkedin.com/sharing/share-offsite/?url=${encodeURIComponent(pageUrl)}`;

        const xBtn = document.getElementById('share-btn-x');
        if (xBtn) xBtn.href = `https://twitter.com/intent/tweet?url=${encodeURIComponent(pageUrl)}&text=${encodeURIComponent(pageTitle)}`;

        const fbBtn = document.getElementById('share-btn-facebook');
        if (fbBtn) fbBtn.href = `https://www.facebook.com/sharer/sharer.php?u=${encodeURIComponent(pageUrl)}`;

        const tgBtn = document.getElementById('share-btn-telegram');
        if (tgBtn) tgBtn.href = `https://t.me/share/url?url=${encodeURIComponent(pageUrl)}&text=${encodeURIComponent(pageTitle)}`;

        const mailBtn = document.getElementById('share-btn-email');
        if (mailBtn) mailBtn.href = `mailto:?subject=${encodeURIComponent(pageTitle)}&body=${encodeURIComponent(pageUrl)}`;

        if (shareLinkInput) shareLinkInput.textContent = pageUrl;

        if (btnOpenShareSheet) {
            btnOpenShareSheet.addEventListener('click', () => openBottomSheet(shareSheet));
        }
        if (btnCloseShareSheet) {
            btnCloseShareSheet.addEventListener('click', () => closeBottomSheet(shareSheet));
        }
        if (backdropShareSheet) {
            backdropShareSheet.addEventListener('click', () => closeBottomSheet(shareSheet));
        }

        if (btnCopyShareLink) {
            btnCopyShareLink.addEventListener('click', () => {
                navigator.clipboard.writeText(pageUrl).then(() => {
                    if (copyBtnText) copyBtnText.textContent = 'Copied! ✓';
                    btnCopyShareLink.classList.add('copied');
                    setTimeout(() => {
                        if (copyBtnText) copyBtnText.textContent = 'Copy Link';
                        btnCopyShareLink.classList.remove('copied');
                    }, 2000);
                }).catch(() => {
                    // Fallback
                });
            });
        }
    }

    // Auto-hide Mobile Sticky Bottom Bar when scrolling into Similar Jobs or Footer
    const mobileBottomBar = document.getElementById('mobileJobBottomBar');
    const similarJobsSection = document.getElementById('similarJobsSection') || document.querySelector('.ref-sidebar-widget');
    const pageFooter = document.querySelector('.footer') || document.querySelector('footer');

    if (mobileBottomBar && (similarJobsSection || pageFooter)) {
        const updateBottomBarVisibility = () => {
            const viewportHeight = window.innerHeight || document.documentElement.clientHeight;
            let hideBar = false;

            if (similarJobsSection) {
                const rect = similarJobsSection.getBoundingClientRect();
                // When top of Similar Jobs enters viewport (within 60px of screen bottom)
                if (rect.top <= (viewportHeight - 60)) {
                    hideBar = true;
                }
            } else if (pageFooter) {
                const footerRect = pageFooter.getBoundingClientRect();
                if (footerRect.top <= (viewportHeight - 40)) {
                    hideBar = true;
                }
            }

            if (hideBar) {
                mobileBottomBar.classList.add('mobile-job-bottom-bar--hidden');
            } else {
                mobileBottomBar.classList.remove('mobile-job-bottom-bar--hidden');
            }
        };

        window.addEventListener('scroll', updateBottomBarVisibility, { passive: true });
        window.addEventListener('resize', updateBottomBarVisibility, { passive: true });
        updateBottomBarVisibility();
    }

    // Close menu when clicking outside
    document.addEventListener('click', (e) => {
        if (shareMenu && !shareMenu.contains(e.target)) {
            closeShareMenu();
        }
    });

    // Close menu on scroll to prevent detached float positions
    window.addEventListener('scroll', closeShareMenu, { passive: true });

    // Weekly Job Alert is now fully dynamic via WhatsApp 3-Day Free Trial (/api/whatsapp/trial-subscribe)

    // Easy Apply Toggle auto-submit handler (CSP compliant)
    const easyApplyToggle = document.getElementById('easy-apply-toggle');
    if (easyApplyToggle) {
        easyApplyToggle.addEventListener('change', () => {
            easyApplyToggle.form.submit();
        });
    }

    // Report Job modal and form submission handler (CSP compliant)
    const reportBtn = document.getElementById("btn-report-job");
    const reportModal = document.getElementById("report-job-modal");
    const cancelBtn = document.getElementById("report-modal-cancel");
    const reportForm = document.getElementById("report-job-form");

    // Custom Toast Notification System (CSP compliant)
    function showToast(message, type = 'success') {
        let container = document.getElementById('toast-container');
        if (!container) {
            container = document.createElement('div');
            container.id = 'toast-container';
            container.style.cssText = 'position: fixed; top: 24px; right: 24px; z-index: 10000; display: flex; flex-direction: column; gap: 12px; max-width: 380px; width: calc(100% - 48px); pointer-events: none;';
            document.body.appendChild(container);
        }

        const toast = document.createElement('div');
        toast.style.cssText = 'padding: 16px 20px; border-radius: 12px; background: #ffffff; box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05); display: flex; align-items: center; gap: 12px; transform: translateY(-20px); opacity: 0; transition: all 0.35s cubic-bezier(0.16, 1, 0.3, 1); pointer-events: auto; border: 1px solid #f1f5f9;';

        const isError = type === 'error';
        const iconColor = isError ? '#ef4444' : '#10b981';
        const iconSvg = isError ?
            `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="${iconColor}" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="8" x2="12" y2="12"></line><line x1="12" y1="16" x2="12.01" y2="16"></line></svg>` :
            `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="${iconColor}" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>`;

        toast.innerHTML = `
            <div style="flex-shrink: 0; display: flex; align-items: center; justify-content: center; width: 36px; height: 36px; border-radius: 8px; background: ${isError ? '#fff5f5' : '#ecfdf5'};">
                ${iconSvg}
            </div>
            <div style="flex-grow: 1; font-size: 0.9rem; font-weight: 600; color: #1e293b; line-height: 1.4;">
                ${message}
            </div>
            <button style="border: none; background: transparent; cursor: pointer; color: #94a3b8; padding: 4px; display: flex; align-items: center; justify-content: center; border-radius: 4px; transition: color 0.2s;" onmouseover="this.style.color='#475569'" onmouseout="this.style.color='#94a3b8'">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>
            </button>
        `;

        toast.querySelector('button').addEventListener('click', () => {
            toast.style.opacity = '0';
            toast.style.transform = 'translateY(-20px)';
            setTimeout(() => toast.remove(), 350);
        });

        container.appendChild(toast);

        // Force reflow and animate in
        toast.offsetHeight;
        toast.style.opacity = '1';
        toast.style.transform = 'translateY(0)';

        setTimeout(() => {
            if (toast.parentNode) {
                toast.style.opacity = '0';
                toast.style.transform = 'translateY(-20px)';
                setTimeout(() => toast.remove(), 350);
            }
        }, 4000);
    }

    if (reportBtn && reportModal) {
        reportBtn.addEventListener("click", () => {
            // Check if already reported twice on client-side
            if (reportBtn.getAttribute("data-already-reported") === "true") {
                showToast("Sorry, you have already reported this job.", "error");
                return;
            }

            reportModal.style.display = "flex";
            setTimeout(() => {
                reportModal.style.opacity = "1";
                reportModal.querySelector(".custom-modal").style.transform = "scale(1)";
            }, 10);
        });

        const closeModal = () => {
            reportModal.style.opacity = "0";
            reportModal.querySelector(".custom-modal").style.transform = "scale(0.95)";
            setTimeout(() => {
                reportModal.style.display = "none";
            }, 250);
        };

        cancelBtn.addEventListener("click", closeModal);

        const backdrop = reportModal.querySelector(".custom-modal-backdrop");
        if (backdrop) {
            backdrop.addEventListener("click", closeModal);
        }

        reportForm.addEventListener("submit", (e) => {
            e.preventDefault();
            const submitBtn = reportForm.querySelector("button[type='submit']");
            const originalText = submitBtn.innerText;
            submitBtn.innerText = "Submitting...";
            submitBtn.disabled = true;

            const formData = new FormData(reportForm);
            const bodyParams = new URLSearchParams(formData);
            const csrfToken = formData.get("csrf_token") || "";

            fetch(reportForm.action, {
                method: "POST",
                headers: {
                    "Content-Type": "application/x-www-form-urlencoded",
                    "X-CSRF-Token": csrfToken
                },
                body: bodyParams.toString()
            })
                .then(res => {
                    if (res.ok) {
                        return res.text().then(msg => ({ ok: true, msg }));
                    }
                    if (res.status === 429) {
                        return res.text().then(msg => ({ ok: false, status: 429, msg }));
                    }
                    if (res.status === 403) {
                        return res.text().then(msg => ({ ok: false, status: 403, msg }));
                    }
                    throw new Error("Failed to submit");
                })
                .then(result => {
                    if (result.ok) {
                        showToast(result.msg, "success");
                        closeModal();
                        reportForm.reset();
                    } else if (result.status === 429) {
                        reportBtn.setAttribute("data-already-reported", "true");
                        showToast(result.msg, "error");
                        closeModal();
                    } else if (result.status === 403) {
                        const ctaContainer = document.querySelector(".job-detail-report-cta");
                        if (ctaContainer) {
                            ctaContainer.style.display = "none";
                        }
                        showToast(result.msg, "error");
                        closeModal();
                    }
                })
                .catch(err => {
                    showToast("Something went wrong. Please try again.", "error");
                })
                .finally(() => {
                    submitBtn.innerText = originalText;
                    submitBtn.disabled = false;
                });
        });
    }

    // Initialize on load
    saveCurrentJobDetail();
    updateViewedTabCount();

    // ==========================================
    // Homepage Company Logo Carousel Slider
    // ==========================================
    const brandSlider = document.getElementById("brand-slider");
    if (brandSlider && brandSlider.children.length > 0) {
        // Clone the first slide and append to the end for seamless infinite forward loop
        const firstSlideClone = brandSlider.firstElementChild.cloneNode(true);
        firstSlideClone.setAttribute("aria-hidden", "true");
        brandSlider.appendChild(firstSlideClone);

        const totalBrandSlides = brandSlider.children.length;
        let currentBrandSlide = 0;
        let brandAutoSlideInterval;

        const goToBrandSlide = (index) => {
            if (index >= totalBrandSlides) {
                brandSlider.style.transition = "none";
                brandSlider.style.transform = "translateX(0%)";
                currentBrandSlide = 0;
                void brandSlider.offsetWidth;
                requestAnimationFrame(() => {
                    brandSlider.style.transition = "";
                    currentBrandSlide = 1;
                    brandSlider.style.transform = `translateX(-100%)`;
                });
                return;
            }
            currentBrandSlide = index;
            brandSlider.style.transition = "";
            brandSlider.style.transform = `translateX(-${currentBrandSlide * 100}%)`;
        };

        const startBrandAutoSlide = () => {
            brandAutoSlideInterval = setInterval(() => {
                let nextSlide = currentBrandSlide + 1;
                goToBrandSlide(nextSlide);
            }, 3200);
        };

        // Seamless transition loop handler
        brandSlider.addEventListener("transitionend", () => {
            // When we finish transitioning forward to the cloned first slide (last index)
            if (currentBrandSlide === totalBrandSlides - 1) {
                brandSlider.style.transition = "none"; // Disable animation instantly
                brandSlider.style.transform = "translateX(0%)"; // Instant snap to S1 (exact same visuals)
                currentBrandSlide = 0;
                void brandSlider.offsetWidth; // Force layout reflow
                requestAnimationFrame(() => {
                    brandSlider.style.transition = ""; // Restore CSS transition for next forward step
                });
            }
        });

        setTimeout(() => {
            startBrandAutoSlide();
        }, 4000);
    }

    // ==========================================
    // Homepage Browse Jobs Tab Switcher
    // ==========================================
    const browseTabButtons = document.querySelectorAll(".browse-tab-btn");
    const browseTabPanels = document.querySelectorAll(".browse-tab-panel");
    if (browseTabButtons.length > 0 && browseTabPanels.length > 0) {
        browseTabButtons.forEach(button => {
            button.addEventListener("click", () => {
                browseTabButtons.forEach(btn => btn.classList.remove("browse-tab-btn--active"));
                browseTabPanels.forEach(panel => panel.classList.remove("browse-tab-panel--active"));

                button.classList.add("browse-tab-btn--active");

                const targetSelector = button.getAttribute("data-tab-target");
                const targetPanel = document.querySelector(targetSelector);
                if (targetPanel) {
                    targetPanel.classList.add("browse-tab-panel--active");
                }
            });
        });
    }

    // ==========================================
    // Locations Directory Page Module
    // ==========================================
    const locationSearchInput = document.getElementById('location-search-input');
    const locationsGrid = document.getElementById('locations-grid');
    if (locationsGrid) {
        const metadata = {
            'england/london': {
                desc: 'Hub of global finance, technological innovation, creative arts, and healthcare. Discover diverse opportunities in the capital city.',
                theme: 'theme-tech'
            },
            'england/manchester': {
                desc: 'Explore software development, media production at MediaCityUK, financial services, and digital startup vacancies in the Northwest.',
                theme: 'theme-finance'
            },
            'england/birmingham': {
                desc: 'Find career openings in engineering, advanced manufacturing, logistics, professional business service desks, and public services.',
                theme: 'theme-health'
            },
            'england/leeds': {
                desc: 'Search UK banking, legal services, digital marketing agencies, and healthcare software systems firms in Yorkshire.',
                theme: 'theme-edu'
            },
            'england/bristol': {
                desc: 'Renowned for aerospace engineering, digital media, research science, environmental technology, and creative startup communities.',
                theme: 'theme-eng'
            },
            'scotland/edinburgh': {
                desc: 'Explore asset management, software development engineering, tourism hospitality, and public sector positions in the Scottish capital.',
                theme: 'theme-const'
            },
            'scotland/glasgow': {
                desc: 'Browse retail commerce, engineering trades, maritime logistics, and public service administration opportunities in Glasgow.',
                theme: 'theme-retail'
            },
            'england/sheffield': {
                desc: 'Search openings in precision engineering, metallurgy research, academic institutions, and health technology companies.',
                theme: 'theme-hosp'
            },
            'england/bradford': {
                desc: 'Explore regional vacancies in chemical processing, food manufacturing, logistics, and digital services firms.',
                theme: 'theme-logistics'
            },
            'england/liverpool': {
                desc: 'A historic shipping port. Discover maritime operations, distribution supply chains, hospitality, and retail vacancies.',
                theme: 'theme-logistics'
            }
        };

        const cards = locationsGrid.querySelectorAll('.directory-card');
        cards.forEach(card => {
            const slug = card.getAttribute('data-location-slug');
            const safeId = slug.replace('/', '-');
            const meta = metadata[slug];
            if (meta) {
                const iconBox = card.querySelector(`#icon-box-${safeId}`);
                const descEl = card.querySelector(`#desc-${safeId}`);
                if (iconBox) {
                    iconBox.className = `directory-card__icon-box ${meta.theme}`;
                }
                if (descEl) {
                    descEl.textContent = meta.desc;
                }
            }
        });

        if (locationSearchInput) {
            locationSearchInput.addEventListener('input', () => {
                const query = locationSearchInput.value.toLowerCase().trim();
                let matchCount = 0;
                cards.forEach(card => {
                    const name = card.getAttribute('data-location-name').toLowerCase();
                    const level = card.getAttribute('data-location-level').toLowerCase();
                    const slug = card.getAttribute('data-location-slug');
                    const desc = metadata[slug] ? metadata[slug].desc.toLowerCase() : '';
                    if (name.includes(query) || level.includes(query) || desc.includes(query)) {
                        card.style.display = 'flex';
                        matchCount++;
                    } else {
                        card.style.display = 'none';
                    }
                });

                let emptyState = document.getElementById('location-empty-state');
                if (matchCount === 0) {
                    if (!emptyState) {
                        emptyState = document.createElement('div');
                        emptyState.id = 'location-empty-state';
                        emptyState.className = 'empty-state text-center';
                        emptyState.style.padding = '40px';
                        emptyState.innerHTML = `
                            <div class="empty-state__icon-box theme-finance" style="width: 64px; height: 64px; border-radius: 16px; display: inline-flex; align-items: center; justify-content: center; margin-bottom: 16px; color: var(--color-success, #00A884); background: rgba(0, 168, 132, 0.08);">
                                <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line></svg>
                            </div>
                            <h3 class="empty-state__title">No locations found</h3>
                            <p class="empty-state__text">We couldn't find any locations matching "${locationSearchInput.value}". Please try another search term.</p>
                        `;
                        locationsGrid.parentNode.appendChild(emptyState);
                    }
                } else {
                    if (emptyState) emptyState.remove();
                }
            });
        }
    }

    // ==========================================
    // Roles Directory Page Module
    // ==========================================
    const roleFallbackContainer = document.getElementById('seo-roles-fallback');
    const roleDirectoryContent = document.getElementById('directory-content');
    const roleNavContainer = document.getElementById('az-nav-container');
    const roleSearchInput = document.getElementById('role-search-input');

    if (roleNavContainer && roleFallbackContainer && roleDirectoryContent) {
        const icons = {
            'software-developer': {
                svg: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"></polyline><polyline points="8 6 2 12 8 18"></polyline></svg>',
                theme: 'theme-tech'
            },
            'software-engineer': {
                svg: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"></polyline><polyline points="8 6 2 12 8 18"></polyline></svg>',
                theme: 'theme-tech'
            },
            'data-analyst': {
                svg: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="20" x2="18" y2="10"></line><line x1="12" y1="20" x2="12" y2="4"></line><line x1="6" y1="20" x2="6" y2="14"></line></svg>',
                theme: 'theme-finance'
            },
            'project-manager': {
                svg: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline><line x1="16" y1="13" x2="8" y2="13"></line><line x1="16" y1="17" x2="8" y2="17"></line><polyline points="10 9 9 9 8 9"></polyline></svg>',
                theme: 'theme-health'
            },
            'accountant': {
                svg: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="2" width="16" height="20" rx="2" ry="2"></rect><line x1="9" y1="22" x2="9" y2="16"></line><line x1="15" y1="22" x2="15" y2="16"></line><line x1="9" y1="16" x2="15" y2="16"></line><path d="M16 8h-4a2 2 0 0 0 0 4h2a2 2 0 0 1 0 4H9"></path></svg>',
                theme: 'theme-edu'
            },
            'nurse': {
                svg: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M22 12h-4l-3 9L9 3l-3 9H2"></path></svg>',
                theme: 'theme-eng'
            },
            'marketing-manager': {
                svg: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M23 7a6 6 0 0 0-6 6v7h6v-7z"></path><path d="M16 8a6 6 0 0 0-6 6v6h6v-6z"></path><path d="M9 9a6 6 0 0 0-6 6v5h6v-5z"></path></svg>',
                theme: 'theme-const'
            },
            'sales': {
                svg: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="9" cy="21" r="1"></circle><circle cx="20" cy="21" r="1"></circle><path d="M1 1h4l2.68 13.39a2 2 0 0 0 2 1.61h9.72a2 2 0 0 0 2-1.61L23 6H6"></path></svg>',
                theme: 'theme-retail'
            },
            'teacher': {
                svg: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"></path><path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"></path></svg>',
                theme: 'theme-hosp'
            },
            'hr': {
                svg: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path><circle cx="9" cy="7" r="4"></circle><path d="M23 21v-2a4 4 0 0 0-3-3.87"></path><path d="M16 3.13a4 4 0 0 1 0 7.75"></path></svg>',
                theme: 'theme-logistics'
            }
        };

        const fallbackCards = Array.from(roleFallbackContainer.getElementsByClassName('directory-card'));
        const rawRoles = fallbackCards.map(card => {
            return {
                slug: card.getAttribute('data-role-slug'),
                name: card.getAttribute('data-role-name'),
                countStr: card.querySelector('.directory-card__badge').textContent
            };
        });

        roleFallbackContainer.style.display = 'none';

        const rolesByLetter = {};
        rawRoles.forEach(role => {
            const firstLetter = role.name.charAt(0).toUpperCase();
            if (!rolesByLetter[firstLetter]) {
                rolesByLetter[firstLetter] = [];
            }
            rolesByLetter[firstLetter].push(role);
        });

        const sortedLetters = Object.keys(rolesByLetter).sort();

        if (roleNavContainer) {
            roleNavContainer.innerHTML = '';
            sortedLetters.forEach(letter => {
                const a = document.createElement('a');
                a.href = `#letter-${letter}`;
                a.className = 'az-nav__item';
                a.textContent = letter;
                a.addEventListener('click', (e) => {
                    e.preventDefault();
                    const target = document.getElementById(`letter-${letter}`);
                    if (target) {
                        target.scrollIntoView({ behavior: 'smooth', block: 'start' });
                    }
                });
                roleNavContainer.appendChild(a);
            });
        }

        const groupsContainer = document.createElement('div');
        groupsContainer.className = 'az-groups';

        sortedLetters.forEach(letter => {
            const groupSection = document.createElement('section');
            groupSection.id = `letter-${letter}`;
            groupSection.className = 'az-group animate-fade-in';
            groupSection.innerHTML = `<h2 class="az-group__letter">${letter}</h2>`;

            const grid = document.createElement('div');
            grid.className = 'grid grid--3';
            groupSection.appendChild(grid);

            rolesByLetter[letter].forEach(role => {
                const card = document.createElement('a');
                card.href = `/job-roles/${role.slug}/`;
                card.className = 'directory-card';
                card.setAttribute('data-search-name', role.name.toLowerCase());

                const iconData = icons[role.slug] || {
                    svg: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="7" width="20" height="14" rx="2" ry="2"></rect><path d="M16 21V5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v16"></path></svg>',
                    theme: 'theme-tech'
                };

                card.innerHTML = `
                    <div class="directory-card__header">
                        <div class="directory-card__icon-box ${iconData.theme}">${iconData.svg}</div>
                        <div class="directory-card__title">${role.name}</div>
                    </div>
                    <div class="directory-card__footer">
                        <span class="directory-card__badge">${role.countStr}</span>
                        <svg class="directory-card__arrow" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
                        </svg>
                    </div>
                `;
                grid.appendChild(card);
            });

            groupsContainer.appendChild(groupSection);
        });

        roleDirectoryContent.appendChild(groupsContainer);

        if (roleSearchInput) {
            roleSearchInput.addEventListener('input', () => {
                const query = roleSearchInput.value.toLowerCase().trim();
                const sections = groupsContainer.querySelectorAll('.az-group');
                let totalMatches = 0;

                sections.forEach(section => {
                    const cards = section.querySelectorAll('.directory-card');
                    let visibleCards = 0;
                    const letter = section.id.replace('letter-', '');

                    cards.forEach(card => {
                        const name = card.getAttribute('data-search-name');
                        if (name.includes(query)) {
                            card.style.display = 'flex';
                            visibleCards++;
                            totalMatches++;
                        } else {
                            card.style.display = 'none';
                        }
                    });

                    const navLink = roleNavContainer ? roleNavContainer.querySelector(`a[href="#letter-${letter}"]`) : null;

                    if (visibleCards > 0) {
                        section.style.display = 'block';
                        if (navLink) navLink.style.display = 'flex';
                    } else {
                        section.style.display = 'none';
                        if (navLink) navLink.style.display = 'none';
                    }
                });

                let emptyState = document.getElementById('directory-empty-state');
                if (totalMatches === 0) {
                    if (!emptyState) {
                        emptyState = document.createElement('div');
                        emptyState.id = 'directory-empty-state';
                        emptyState.className = 'empty-state text-center';
                        emptyState.style.padding = '40px';
                        emptyState.innerHTML = `
                            <div class="empty-state__icon-box theme-finance" style="width: 64px; height: 64px; border-radius: 16px; display: inline-flex; align-items: center; justify-content: center; margin-bottom: 16px; color: var(--color-success, #00A884); background: rgba(0, 168, 132, 0.08);">
                                <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line></svg>
                            </div>
                            <h3 class="empty-state__title">No job roles found</h3>
                            <p class="empty-state__text">We couldn't find any roles matching "${roleSearchInput.value}". Please try another search term.</p>
                        `;
                        roleDirectoryContent.appendChild(emptyState);
                    }
                } else {
                    if (emptyState) emptyState.remove();
                }
            });
        }
    }

    // ==========================================
    // Industries Directory Page Module
    // ==========================================
    const industrySearchInput = document.getElementById('industry-search-input');
    const industriesGrid = document.getElementById('industries-grid');
    if (industriesGrid) {
        const metadata = {
            'technology': {
                desc: 'Innovate with software development, data science, cybersecurity, systems engineering, cloud architecture, and technical support positions across the UK.',
                theme: 'theme-tech',
                icon: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"></rect><line x1="8" y1="21" x2="16" y2="21"></line><line x1="12" y1="17" x2="12" y2="21"></line></svg>'
            },
            'finance': {
                desc: 'Explore accounting, investment banking, auditing, financial consultancy, risk management, and insurance roles with competitive salary rates.',
                theme: 'theme-finance',
                icon: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M18 7c0-5.333-8-5.333-8 0v10a2 2 0 0 0 2 2h6"></path><path d="M6 13h10"></path></svg>'
            },
            'healthcare': {
                desc: 'Deliver vital medical care. Browse nursing, general practitioner, pharmacy, clinical technology, and healthcare support opportunities.',
                theme: 'theme-health',
                icon: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M22 12h-4l-3 9L9 3l-3 9H2"></path></svg>'
            },
            'education': {
                desc: 'Inspire future generations. Find primary and secondary school teaching, special educational needs (SEN), tutoring, and lecturing positions.',
                theme: 'theme-edu',
                icon: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"></path><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"></path></svg>'
            },
            'engineering': {
                desc: 'Engineer solutions. Discover mechanical, electrical, structural, civil, chemical, and aerospace engineering positions with leading firms.',
                theme: 'theme-eng',
                icon: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"></circle><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"></path></svg>'
            },
            'construction': {
                desc: 'Build key infrastructure. Find quantity surveying, site management, trade contractor (plumbing, electrical, carpentry), and general labour jobs.',
                theme: 'theme-const',
                icon: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect><line x1="9" y1="3" x2="9" y2="21"></line><line x1="15" y1="3" x2="15" y2="21"></line><line x1="3" y1="9" x2="21" y2="9"></line><line x1="3" y1="15" x2="21" y2="15"></line></svg>'
            },
            'retail': {
                desc: 'Connect with UK consumers. Search retail store management, sales assistant, buying, visual merchandising, and customer service roles.',
                theme: 'theme-retail',
                icon: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M6 2L3 6v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V6l-3-4z"></path><line x1="3" y1="6" x2="21" y2="6"></line><path d="M16 10a4 4 0 0 1-8 0"></path></svg>'
            },
            'hospitality': {
                desc: 'Deliver premium service. Browse hotel management, head chef, catering staff, event coordination, bar work, and travel host vacancies.',
                theme: 'theme-hosp',
                icon: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"></path><path d="M13.73 21a2 2 0 0 1-3.46 0"></path></svg>'
            },
            'logistics': {
                desc: 'Manage global supply chains. Find HGV driving, warehouse operation, distribution planning, inventory control, and transport management positions.',
                theme: 'theme-logistics',
                icon: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><rect x="1" y="3" width="15" height="13"></rect><polygon points="16 8 20 8 23 11 23 16 16 16 16 8"></polygon><circle cx="5.5" cy="18.5" r="2.5"></circle><circle cx="18.5" cy="18.5" r="2.5"></circle></svg>'
            }
        };

        const cards = industriesGrid.querySelectorAll('.directory-card');
        cards.forEach(card => {
            const slug = card.getAttribute('data-industry-slug');
            const meta = metadata[slug];
            if (meta) {
                const iconBox = card.querySelector(`#icon-box-${slug}`);
                const descEl = card.querySelector(`#desc-${slug}`);
                if (iconBox) {
                    iconBox.className = `directory-card__icon-box ${meta.theme}`;
                    iconBox.innerHTML = meta.icon;
                }
                if (descEl) {
                    descEl.textContent = meta.desc;
                }
            }
        });

        if (industrySearchInput) {
            industrySearchInput.addEventListener('input', () => {
                const query = industrySearchInput.value.toLowerCase().trim();
                let matchCount = 0;
                cards.forEach(card => {
                    const name = card.getAttribute('data-industry-name').toLowerCase();
                    const slug = card.getAttribute('data-industry-slug');
                    const desc = metadata[slug] ? metadata[slug].desc.toLowerCase() : '';
                    if (name.includes(query) || desc.includes(query)) {
                        card.style.display = 'flex';
                        matchCount++;
                    } else {
                        card.style.display = 'none';
                    }
                });

                let emptyState = document.getElementById('industry-empty-state');
                if (matchCount === 0) {
                    if (!emptyState) {
                        emptyState = document.createElement('div');
                        emptyState.id = 'industry-empty-state';
                        emptyState.className = 'empty-state text-center';
                        emptyState.style.padding = '40px';
                        emptyState.innerHTML = `
                            <div class="empty-state__icon-box theme-finance" style="width: 64px; height: 64px; border-radius: 16px; display: inline-flex; align-items: center; justify-content: center; margin-bottom: 16px; color: var(--color-success, #00A884); background: rgba(0, 168, 132, 0.08);">
                                <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line></svg>
                            </div>
                            <h3 class="empty-state__title">No industry sectors found</h3>
                            <p class="empty-state__text">We couldn't find any sectors matching "${industrySearchInput.value}". Please try another search term.</p>
                        `;
                        industriesGrid.parentNode.appendChild(emptyState);
                    }
                } else {
                    if (emptyState) emptyState.remove();
                }
            });
        }
    }

    // ==========================================
    // Companies Directory Page Module
    // ==========================================
    const companySearchInput = document.getElementById('company-search');
    const companyList = document.getElementById('companies-list');
    const companyLoadMoreBtn = document.getElementById('load-more-btn');
    const companyLoadMoreContainer = document.getElementById('load-more-container');
    const companySpinner = document.getElementById('load-more-spinner');
    const companyBtnText = document.getElementById('load-more-text');

    if (companyList) {
        const cards = Array.from(companyList.getElementsByClassName('company-card'));

        if (companyLoadMoreBtn && companyLoadMoreContainer) {
            const hiddenCards = Array.from(document.querySelectorAll('.company-card--hidden'));
            let currentIndex = 0;
            const batchSize = 18;

            const loadMore = () => {
                if (companyLoadMoreBtn.disabled) return;

                if (companySpinner) companySpinner.style.display = 'inline-block';
                companyLoadMoreBtn.disabled = true;
                companyLoadMoreBtn.style.opacity = '0.8';
                if (companyBtnText) companyBtnText.textContent = 'Loading...';

                setTimeout(() => {
                    const nextBatch = hiddenCards.slice(currentIndex, currentIndex + batchSize);
                    nextBatch.forEach(card => {
                        card.style.display = 'flex';
                        card.classList.remove('company-card--hidden');
                        card.style.animation = 'fadeInUp 0.4s ease forwards';
                    });

                    currentIndex += batchSize;

                    if (companySpinner) companySpinner.style.display = 'none';
                    companyLoadMoreBtn.disabled = false;
                    companyLoadMoreBtn.style.opacity = '1';
                    if (companyBtnText) companyBtnText.textContent = 'Load More Companies';

                    if (currentIndex >= hiddenCards.length) {
                        companyLoadMoreContainer.style.display = 'none';
                        observer.disconnect();
                    }
                }, 600);
            };

            companyLoadMoreBtn.addEventListener('click', loadMore);

            const observer = new IntersectionObserver((entries) => {
                entries.forEach(entry => {
                    if (entry.isIntersecting) {
                        loadMore();
                    }
                });
            }, {
                rootMargin: '100px'
            });

            observer.observe(companyLoadMoreBtn);
        }

        if (companySearchInput) {
            companySearchInput.addEventListener('input', (e) => {
                const query = e.target.value.toLowerCase().trim();

                if (query === '') {
                    cards.forEach((card, index) => {
                        if (index < 18) {
                            card.style.display = 'flex';
                            card.classList.remove('company-card--hidden');
                        } else {
                            card.style.display = 'none';
                            card.classList.add('company-card--hidden');
                        }
                    });
                    if (companyLoadMoreContainer) {
                        companyLoadMoreContainer.style.display = 'block';
                        if (companySpinner) companySpinner.style.display = 'none';
                        if (companyLoadMoreBtn) {
                            companyLoadMoreBtn.disabled = false;
                            companyLoadMoreBtn.style.opacity = '1';
                        }
                        if (companyBtnText) companyBtnText.textContent = 'Load More Companies';
                    }
                } else {
                    cards.forEach(card => {
                        const name = card.getAttribute('data-name').toLowerCase();
                        if (name.includes(query)) {
                            card.style.display = 'flex';
                            card.classList.remove('company-card--hidden');
                        } else {
                            card.style.display = 'none';
                        }
                    });
                    if (companyLoadMoreContainer) {
                        companyLoadMoreContainer.style.display = 'none';
                    }
                }
            });
        }
    }

    // --- Dual View Mode Switcher (Pagination vs Infinite Scroll) ---
    function initViewModeController() {
        const btnsPagination = document.querySelectorAll('.btn-mode-pagination');
        const btnsInfinite = document.querySelectorAll('.btn-mode-infinite');
        const paginationWrap = document.getElementById('pagination-mode-wrap');
        const infiniteWrap = document.getElementById('infinite-mode-wrap');

        const isMobile = window.innerWidth < 1024;
        let currentMode = isMobile ? 'infinite' : (localStorage.getItem('JobyUK_view_mode') || 'pagination');

        function applyMode(mode) {
            currentMode = (window.innerWidth < 1024) ? 'infinite' : mode;
            if (window.innerWidth >= 1024) {
                localStorage.setItem('JobyUK_view_mode', mode);
            }

            const urlParams = new URLSearchParams(window.location.search);
            const pageFromUrl = parseInt(urlParams.get('page') || '1', 10);
            const container = document.getElementById('infinite-scroll-container');
            const containerPage = container ? parseInt(container.getAttribute('data-current-page') || '1', 10) : 1;

            if (currentMode === 'infinite') {
                btnsInfinite.forEach(btn => btn.classList.add('active'));
                btnsPagination.forEach(btn => btn.classList.remove('active'));

                if (paginationWrap) paginationWrap.style.display = 'none';
                if (infiniteWrap) infiniteWrap.style.display = 'block';

                // Sync current page from URL / container when entering infinite mode
                const activePage = pageFromUrl > containerPage ? pageFromUrl : containerPage;
                if (container && activePage > 1) {
                    container.setAttribute('data-current-page', activePage.toString());
                }

                initInfiniteScrollObserver();
            } else {
                btnsPagination.forEach(btn => btn.classList.add('active'));
                btnsInfinite.forEach(btn => btn.classList.remove('active'));

                if (paginationWrap) paginationWrap.style.display = 'block';
                if (infiniteWrap) infiniteWrap.style.display = 'none';

                // If active page is > 1, update URL so pagination links match
                const activePage = pageFromUrl > containerPage ? pageFromUrl : containerPage;
                if (activePage > 1) {
                    const currentUrl = new URL(window.location.href);
                    currentUrl.searchParams.set('page', activePage);
                    history.replaceState(null, '', currentUrl.toString());
                }
            }
        }

        btnsPagination.forEach(btn => {
            btn.addEventListener('click', () => applyMode('pagination'));
        });
        btnsInfinite.forEach(btn => {
            btn.addEventListener('click', () => applyMode('infinite'));
        });

        // Check total pages to hide sticky bar if pagination is complete or not needed
        const stickyBar = document.getElementById('sticky-view-mode-bar');
        const containerEl = document.getElementById('infinite-scroll-container');
        const currentPageCount = containerEl ? parseInt(containerEl.getAttribute('data-current-page') || '1', 10) : 1;
        const totalPagesCount = containerEl ? parseInt(containerEl.getAttribute('data-total-pages') || '1', 10) : 1;
        const hasPagination = document.querySelector('#pagination-mode-wrap .pagination') !== null;

        if (stickyBar) {
            stickyBar.style.display = 'flex';
        }

        // Apply mode on load
        applyMode(currentMode);
    }

    // Hide Floating Bar when scrolling near footer card (prevent overlapping footer card)
    function initFooterBarObserver() {
        const stickyBar = document.getElementById('sticky-view-mode-bar');
        const footerCard = document.querySelector('.footer-card') || document.getElementById('site-footer') || document.querySelector('footer');
        if (!stickyBar || !footerCard) return;

        function checkFooterDistance() {
            if (window.innerWidth <= 768) {
                // On mobile devices, keep sticky trial bottom bar always docked and visible
                stickyBar.classList.remove('glass-bar--hidden');
                return;
            }
            const footerRect = footerCard.getBoundingClientRect();
            const windowHeight = window.innerHeight;

            // Hide bar when top of footer card touches bottom of screen (0px offset)
            if (footerRect.top <= windowHeight) {
                stickyBar.classList.add('glass-bar--hidden');
            } else {
                stickyBar.classList.remove('glass-bar--hidden');
            }
        }

        window.removeEventListener('scroll', checkFooterDistance);
        window.addEventListener('scroll', checkFooterDistance, { passive: true });
        window.addEventListener('resize', checkFooterDistance, { passive: true });
        checkFooterDistance();
    }

    let infiniteObserver = null;
    let isLoadingMoreJobs = false;

    async function fetchNextPageJobs() {
        const container = document.getElementById('infinite-scroll-container');
        const btn = document.getElementById('load-more-jobs-btn');
        const stickyBar = document.getElementById('sticky-view-mode-bar');

        if (!container || !btn || isLoadingMoreJobs) return;

        const urlParams = new URLSearchParams(window.location.search);
        const pageFromUrl = parseInt(urlParams.get('page') || '1', 10);
        const containerPage = parseInt(container.getAttribute('data-current-page') || '1', 10);
        const currentPage = Math.max(pageFromUrl, containerPage);

        const totalPages = parseInt(container.getAttribute('data-total-pages') || '1', 10);
        const totalJobs = container.getAttribute('data-total-jobs') || '';

        if (currentPage >= totalPages) {
            container.innerHTML = '';
            container.style.display = 'none';
            if (stickyBar && window.innerWidth > 768) stickyBar.style.display = 'none';
            return;
        }

        isLoadingMoreJobs = true;
        const spinner = btn.querySelector('.load-more-spinner');
        const btnText = btn.querySelector('.load-more-text');
        const arrow = btn.querySelector('.load-more-arrow');

        if (spinner) spinner.style.display = 'inline-block';
        if (btnText) btnText.textContent = 'Loading More...';
        if (arrow) arrow.style.display = 'none';
        btn.disabled = true;

        const nextPage = currentPage + 1;
        const currentUrl = new URL(window.location.href);
        currentUrl.searchParams.set('page', nextPage);

        try {
            const response = await fetch(currentUrl.toString());
            if (!response.ok) throw new Error('Failed to fetch next page');
            const html = await response.text();

            const parser = new DOMParser();
            const doc = parser.parseFromString(html, 'text/html');

            const newJobs = doc.querySelectorAll('.listing-jobs > .job-card');
            const jobsContainer = document.querySelector('.listing-jobs');

            if (jobsContainer && newJobs.length > 0) {
                newJobs.forEach((card, index) => {
                    card.classList.add('job-card-append-anim');
                    card.style.animationDelay = `${index * 0.05}s`;
                    jobsContainer.appendChild(card);
                });

                // Refresh ads for newly appended items
                if (typeof window.JobyUKRefreshAds === 'function') {
                    window.JobyUKRefreshAds();
                }

                // Update showing counter
                const showingCountEl = document.getElementById('jobs-showing-count');
                if (showingCountEl) {
                    const allCards = jobsContainer.querySelectorAll('.job-card');
                    showingCountEl.textContent = allCards.length;
                }

                // SYNCHRONIZE PAGINATION MODE HTML WITH FETCHED PAGE
                const newPagination = doc.querySelector('#pagination-mode-wrap');
                const oldPagination = document.querySelector('#pagination-mode-wrap');
                if (newPagination && oldPagination) {
                    oldPagination.innerHTML = newPagination.innerHTML;
                    if (typeof bindListingFilters === 'function') {
                        bindListingFilters();
                    }
                }

                // Re-mark viewed cards
                if (typeof markViewedJobsOnPage === 'function') {
                    markViewedJobsOnPage();
                }

                container.setAttribute('data-current-page', nextPage.toString());
                history.replaceState(null, '', currentUrl.toString());

                if (nextPage >= totalPages) {
                    container.innerHTML = '';
                    container.style.display = 'none';
                    // PAGINATION / INFINITE SCROLL COMPLETE: HIDE STICKY BAR ON DESKTOP ONLY!
                    if (stickyBar && window.innerWidth > 768) stickyBar.style.display = 'none';
                } else {
                    if (spinner) spinner.style.display = 'none';
                    if (btnText) btnText.textContent = 'Load More Jobs';
                    if (arrow) arrow.style.display = 'inline-block';
                    btn.disabled = false;
                }
            } else {
                container.innerHTML = '';
                container.style.display = 'none';
                if (stickyBar) stickyBar.style.display = 'none';
            }
        } catch (err) {
            console.error('Infinite scroll fetch error:', err);
            if (spinner) spinner.style.display = 'none';
            if (btnText) btnText.textContent = 'Try Again';
            if (arrow) arrow.style.display = 'inline-block';
            btn.disabled = false;
        } finally {
            isLoadingMoreJobs = false;
        }
    }

    function initInfiniteScrollObserver() {
        const btn = document.getElementById('load-more-jobs-btn');
        if (!btn) return;

        // Manual click listener
        if (!btn.dataset.hasClickListener) {
            btn.dataset.hasClickListener = 'true';
            btn.addEventListener('click', fetchNextPageJobs);
        }

        // IntersectionObserver for auto-trigger when scrolling into view
        if ('IntersectionObserver' in window) {
            if (infiniteObserver) {
                infiniteObserver.disconnect();
            }
            infiniteObserver = new IntersectionObserver((entries) => {
                entries.forEach((entry) => {
                    if (entry.isIntersecting) {
                        const isMobile = window.innerWidth < 1024;
                        const currentMode = isMobile ? 'infinite' : (localStorage.getItem('JobyUK_view_mode') || 'pagination');
                        if (currentMode === 'infinite') {
                            fetchNextPageJobs();
                        }
                    }
                });
            }, { rootMargin: '350px' });

            infiniteObserver.observe(btn);
        }
    }

    // --- Global Job Card Click Handler (Make Entire Card Clickable) ---
    document.addEventListener('click', (e) => {
        const card = e.target.closest('.job-card');
        if (!card) return;

        if (e.target.closest('a') || e.target.closest('button')) return;

        const titleLink = card.querySelector('.job-card__title a');
        if (titleLink && titleLink.href) {
            if (e.ctrlKey || e.metaKey) {
                window.open(titleLink.href, '_blank');
            } else {
                window.location.href = titleLink.href;
            }
        }
    });

    // --- Job Roles Directory Auto-Trigger Load More ---
    let isLoadingMoreRoles = false;
    let infiniteRolesObserver = null;

    async function fetchNextPageRoles() {
        const container = document.getElementById('infinite-roles-container');
        const btn = document.getElementById('load-more-roles-btn');
        if (!container || !btn || isLoadingMoreRoles) return;

        const currentPage = parseInt(container.getAttribute('data-current-page') || '1', 10);
        const totalPages = parseInt(container.getAttribute('data-total-pages') || '1', 10);
        const query = container.getAttribute('data-query') || '';
        const letter = container.getAttribute('data-letter') || '';

        if (currentPage >= totalPages) {
            container.innerHTML = '';
            container.style.display = 'none';
            return;
        }

        isLoadingMoreRoles = true;
        const spinner = btn.querySelector('.load-more-spinner');
        const btnText = btn.querySelector('.load-more-text');
        const arrow = btn.querySelector('.load-more-arrow');

        if (spinner) spinner.style.display = 'inline-block';
        if (btnText) btnText.textContent = 'Loading Roles...';
        if (arrow) arrow.style.display = 'none';
        btn.disabled = true;

        const nextPage = currentPage + 1;
        const targetUrl = new URL(window.location.href);
        targetUrl.searchParams.set('page', nextPage);
        if (query) targetUrl.searchParams.set('q', query);
        if (letter) targetUrl.searchParams.set('letter', letter);

        try {
            const response = await fetch(targetUrl.toString());
            if (!response.ok) throw new Error('Failed to fetch next page of roles');
            const html = await response.text();

            const parser = new DOMParser();
            const doc = parser.parseFromString(html, 'text/html');

            const newRoles = doc.querySelectorAll('#seo-roles-fallback > .directory-card');
            const rolesGrid = document.querySelector('#seo-roles-fallback');

            if (rolesGrid && newRoles.length > 0) {
                newRoles.forEach((card, index) => {
                    card.classList.add('animate-fade-in');
                    card.style.animationDelay = `${index * 0.03}s`;
                    rolesGrid.appendChild(card);
                });

                container.setAttribute('data-current-page', nextPage.toString());
                history.replaceState(null, '', targetUrl.toString());

                if (nextPage >= totalPages) {
                    container.innerHTML = '';
                    container.style.display = 'none';
                } else {
                    if (spinner) spinner.style.display = 'none';
                    if (btnText) btnText.textContent = 'Load More Job Roles';
                    if (arrow) arrow.style.display = 'inline-block';
                    btn.disabled = false;
                }
            } else {
                container.innerHTML = '';
                container.style.display = 'none';
            }
        } catch (err) {
            console.error('Infinite roles fetch error:', err);
            if (spinner) spinner.style.display = 'none';
            if (btnText) btnText.textContent = 'Try Again';
            if (arrow) arrow.style.display = 'inline-block';
            btn.disabled = false;
        } finally {
            isLoadingMoreRoles = false;
        }
    }

    function initInfiniteRolesObserver() {
        const btn = document.getElementById('load-more-roles-btn');
        if (!btn) return;

        if (!btn.dataset.hasClickListener) {
            btn.dataset.hasClickListener = 'true';
            btn.addEventListener('click', fetchNextPageRoles);
        }

        if ('IntersectionObserver' in window) {
            if (infiniteRolesObserver) {
                infiniteRolesObserver.disconnect();
            }
            infiniteRolesObserver = new IntersectionObserver((entries) => {
                entries.forEach((entry) => {
                    if (entry.isIntersecting) {
                        fetchNextPageRoles();
                    }
                });
            }, { rootMargin: '300px' });

            infiniteRolesObserver.observe(btn);
        }
    }
    initInfiniteRolesObserver();

    // Auto-scroll active A-Z letter into view on page load and on click
    function initActiveAzLetterScroller() {
        const azWrap = document.querySelector('.glass-roles-az-wrap');
        const activeAzLink = document.querySelector('.glass-roles-az-wrap .glass-az-link.active');
        if (activeAzLink && azWrap) {
            setTimeout(() => {
                const wrapRect = azWrap.getBoundingClientRect();
                const linkRect = activeAzLink.getBoundingClientRect();
                const offset = (linkRect.left - wrapRect.left) + azWrap.scrollLeft - (wrapRect.width / 2) + (linkRect.width / 2);
                azWrap.scrollTo({
                    left: Math.max(0, offset),
                    behavior: 'smooth'
                });
            }, 60);
        }
    }
    initActiveAzLetterScroller();

    // --- UK Salary Guides Directory Auto-Trigger Load More ---
    let isLoadingMoreSalaries = false;
    let infiniteSalariesObserver = null;

    async function fetchNextPageSalaries() {
        const container = document.getElementById('infinite-salaries-container');
        const btn = document.getElementById('load-more-salaries-btn');
        if (!container || !btn || isLoadingMoreSalaries) return;

        const currentPage = parseInt(container.getAttribute('data-current-page') || '1', 10);
        const totalPages = parseInt(container.getAttribute('data-total-pages') || '1', 10);
        const query = container.getAttribute('data-query') || '';
        const letter = container.getAttribute('data-letter') || '';

        if (currentPage >= totalPages) {
            container.innerHTML = '';
            container.style.display = 'none';
            return;
        }

        isLoadingMoreSalaries = true;
        const spinner = btn.querySelector('.load-more-spinner');
        const btnText = btn.querySelector('.load-more-text');
        const arrow = btn.querySelector('.load-more-arrow');

        if (spinner) spinner.style.display = 'inline-block';
        if (btnText) btnText.textContent = 'Loading Salary Guides...';
        if (arrow) arrow.style.display = 'none';
        btn.disabled = true;

        const nextPage = currentPage + 1;
        const targetUrl = new URL(window.location.href);
        targetUrl.searchParams.set('page', nextPage);
        if (query) targetUrl.searchParams.set('q', query);
        if (letter) targetUrl.searchParams.set('letter', letter);

        try {
            const response = await fetch(targetUrl.toString());
            if (!response.ok) throw new Error('Failed to fetch next page of salary guides');
            const html = await response.text();

            const parser = new DOMParser();
            const doc = parser.parseFromString(html, 'text/html');

            const newGuides = doc.querySelectorAll('#salary-grid > .directory-card');
            const salaryGrid = document.querySelector('#salary-grid');

            if (salaryGrid && newGuides.length > 0) {
                newGuides.forEach((card, index) => {
                    card.classList.add('animate-fade-in');
                    card.style.animationDelay = `${index * 0.03}s`;
                    salaryGrid.appendChild(card);
                });

                container.setAttribute('data-current-page', nextPage.toString());
                history.replaceState(null, '', targetUrl.toString());

                if (nextPage >= totalPages) {
                    container.innerHTML = '';
                    container.style.display = 'none';
                } else {
                    if (spinner) spinner.style.display = 'none';
                    if (btnText) btnText.textContent = 'Load More Salary Guides';
                    if (arrow) arrow.style.display = 'inline-block';
                    btn.disabled = false;
                }
            } else {
                container.innerHTML = '';
                container.style.display = 'none';
            }
        } catch (err) {
            console.error('Infinite salaries fetch error:', err);
            if (spinner) spinner.style.display = 'none';
            if (btnText) btnText.textContent = 'Try Again';
            if (arrow) arrow.style.display = 'inline-block';
            btn.disabled = false;
        } finally {
            isLoadingMoreSalaries = false;
        }
    }

    function initInfiniteSalariesObserver() {
        const btn = document.getElementById('load-more-salaries-btn');
        if (!btn) return;

        if (!btn.dataset.hasClickListener) {
            btn.dataset.hasClickListener = 'true';
            btn.addEventListener('click', fetchNextPageSalaries);
        }

        if ('IntersectionObserver' in window) {
            if (infiniteSalariesObserver) {
                infiniteSalariesObserver.disconnect();
            }
            infiniteSalariesObserver = new IntersectionObserver((entries) => {
                entries.forEach((entry) => {
                    if (entry.isIntersecting) {
                        fetchNextPageSalaries();
                    }
                });
            }, { rootMargin: '300px' });

            infiniteSalariesObserver.observe(btn);
        }
    }

    function initStickyHeaders() {
        const headers = document.querySelectorAll('.glass-roles-sticky-header, .glass-companies-sticky-header, .glass-industries-sticky-header');
        if (!headers.length) return;

        function checkSticky() {
            headers.forEach(header => {
                const rect = header.getBoundingClientRect();
                if (rect.top <= 71) {
                    header.classList.add('is-stuck');
                } else {
                    header.classList.remove('is-stuck');
                }
            });
        }

        window.removeEventListener('scroll', checkSticky);
        window.addEventListener('scroll', checkSticky, { passive: true });
        checkSticky();
    }

    // ==========================================
    // Unified Instant Real-Time Directory Search
    // ==========================================

    // 1. Job Roles Instant Search (Debounced Server-Wide AJAX Search across 2,300+ Roles)
    function initRoleSearchInstant() {
        const input = document.getElementById('role-search-input');
        const grid = document.getElementById('seo-roles-fallback');
        const loadMoreWrap = document.getElementById('infinite-roles-wrap');
        const form = input ? input.closest('form') : null;

        if (!input || !grid) return;
        if (form) {
            form.addEventListener('submit', (e) => e.preventDefault());
        }

        let debounceTimer = null;
        let abortController = null;

        input.addEventListener('input', (e) => {
            const query = e.target.value.trim();

            if (debounceTimer) clearTimeout(debounceTimer);
            if (abortController) abortController.abort();

            debounceTimer = setTimeout(async () => {
                abortController = new AbortController();
                const targetUrl = new URL(window.location.origin + '/job-roles/');
                if (query) {
                    targetUrl.searchParams.set('q', query);
                }

                try {
                    const response = await fetch(targetUrl.toString(), { signal: abortController.signal });
                    if (!response.ok) return;
                    const html = await response.text();

                    const parser = new DOMParser();
                    const doc = parser.parseFromString(html, 'text/html');

                    const newGrid = doc.getElementById('seo-roles-fallback');
                    const currentGrid = document.getElementById('seo-roles-fallback');
                    if (newGrid && currentGrid) {
                        currentGrid.innerHTML = newGrid.innerHTML;
                    }

                    const newLoadMore = doc.getElementById('infinite-roles-wrap');
                    const currentLoadMore = document.getElementById('infinite-roles-wrap');
                    if (newLoadMore && currentLoadMore) {
                        currentLoadMore.innerHTML = newLoadMore.innerHTML;
                        currentLoadMore.style.display = '';
                    } else if (currentLoadMore) {
                        currentLoadMore.style.display = 'none';
                    }

                    history.replaceState(null, '', targetUrl.toString());

                    // Re-initialize infinite observer for fresh cards
                    if (typeof initInfiniteRolesObserver === 'function') {
                        initInfiniteRolesObserver();
                    }
                } catch (err) {
                    if (err.name !== 'AbortError') {
                        console.error('Role search error:', err);
                    }
                }
            }, 120);
        });
    }

    // 2. Salary Guides Instant Search (Debounced Server-Wide AJAX Search)
    function initSalarySearchInstant() {
        const input = document.getElementById('salary-search-input');
        const grid = document.getElementById('salary-grid');
        const loadMoreWrap = document.getElementById('infinite-salaries-wrap');
        const form = input ? input.closest('form') : null;

        if (!input || !grid) return;
        if (form) {
            form.addEventListener('submit', (e) => e.preventDefault());
        }

        let debounceTimer = null;
        let abortController = null;

        input.addEventListener('input', (e) => {
            const query = e.target.value.trim();

            if (debounceTimer) clearTimeout(debounceTimer);
            if (abortController) abortController.abort();

            debounceTimer = setTimeout(async () => {
                abortController = new AbortController();
                const targetUrl = new URL(window.location.origin + '/salaries/');
                if (query) {
                    targetUrl.searchParams.set('q', query);
                }

                try {
                    const response = await fetch(targetUrl.toString(), { signal: abortController.signal });
                    if (!response.ok) return;
                    const html = await response.text();

                    const parser = new DOMParser();
                    const doc = parser.parseFromString(html, 'text/html');

                    const newGrid = doc.getElementById('salary-grid');
                    const currentGrid = document.getElementById('salary-grid');
                    if (newGrid && currentGrid) {
                        currentGrid.innerHTML = newGrid.innerHTML;
                    }

                    const newLoadMore = doc.getElementById('infinite-salaries-wrap');
                    const currentLoadMore = document.getElementById('infinite-salaries-wrap');
                    if (newLoadMore && currentLoadMore) {
                        currentLoadMore.innerHTML = newLoadMore.innerHTML;
                        currentLoadMore.style.display = '';
                    } else if (currentLoadMore) {
                        currentLoadMore.style.display = 'none';
                    }

                    history.replaceState(null, '', targetUrl.toString());

                    if (typeof initInfiniteSalariesObserver === 'function') {
                        initInfiniteSalariesObserver();
                    }
                } catch (err) {
                    if (err.name !== 'AbortError') {
                        console.error('Salary search error:', err);
                    }
                }
            }, 120);
        });
    }

    // 3. Locations Directory Instant Search
    function initLocationSearchInstant() {
        const input = document.getElementById('location-search-input');
        const grid = document.getElementById('locations-grid');

        if (!input || !grid) return;

        const cards = Array.from(grid.querySelectorAll('.directory-card'));

        let emptyMsg = grid.querySelector('.empty-state');
        if (!emptyMsg) {
            emptyMsg = document.createElement('div');
            emptyMsg.className = 'empty-state animate-fade-in';
            emptyMsg.style.cssText = 'text-align: center; padding: 48px 0; grid-column: 1 / -1; display: none;';
            emptyMsg.innerHTML = `
                <div style="width: 64px; height: 64px; border-radius: 16px; display: inline-flex; align-items: center; justify-content: center; margin-bottom: 16px; color: #0A66C2; background: rgba(10, 102, 194, 0.08);">
                    <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 10c0 7-9 13-9 13s-9-6-9-13a9 9 0 0 1 18 0z"></path><circle cx="12" cy="10" r="3"></circle></svg>
                </div>
                <h3 class="empty-state__title" style="font-size: 1.25rem; font-weight: 700; color: #0f172a; margin-bottom: 8px;">No locations matching your search</h3>
                <p class="empty-state__text" style="color: #64748b; font-size: 0.95rem;">Try searching for another city, county, or region.</p>
            `;
            grid.appendChild(emptyMsg);
        }

        input.addEventListener('input', (e) => {
            const query = e.target.value.toLowerCase().trim();
            let visibleCount = 0;

            cards.forEach(card => {
                const name = (card.getAttribute('data-location-name') || card.textContent || '').toLowerCase();
                if (query === '' || name.includes(query)) {
                    card.style.display = 'flex';
                    visibleCount++;
                } else {
                    card.style.display = 'none';
                }
            });

            if (emptyMsg) {
                emptyMsg.style.display = visibleCount === 0 ? 'block' : 'none';
            }
        });
    }

    // 4. Industries Directory Instant Search
    function initIndustrySearchInstant() {
        const input = document.getElementById('industry-search-input');
        const grid = document.getElementById('industries-grid');

        if (!input || !grid) return;

        const cards = Array.from(grid.querySelectorAll('.directory-card'));

        let emptyMsg = grid.querySelector('.empty-state');
        if (!emptyMsg) {
            emptyMsg = document.createElement('div');
            emptyMsg.className = 'empty-state animate-fade-in';
            emptyMsg.style.cssText = 'text-align: center; padding: 48px 0; grid-column: 1 / -1; display: none;';
            emptyMsg.innerHTML = `
                <div style="width: 64px; height: 64px; border-radius: 16px; display: inline-flex; align-items: center; justify-content: center; margin-bottom: 16px; color: #0A66C2; background: rgba(10, 102, 194, 0.08);">
                    <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"></rect><line x1="8" y1="21" x2="16" y2="21"></line><line x1="12" y1="17" x2="12" y2="21"></line></svg>
                </div>
                <h3 class="empty-state__title" style="font-size: 1.25rem; font-weight: 700; color: #0f172a; margin-bottom: 8px;">No industry sectors matching your search</h3>
                <p class="empty-state__text" style="color: #64748b; font-size: 0.95rem;">Try searching for another industry or sector keyword.</p>
            `;
            grid.appendChild(emptyMsg);
        }

        input.addEventListener('input', (e) => {
            const query = e.target.value.toLowerCase().trim();
            let visibleCount = 0;

            cards.forEach(card => {
                const name = (card.getAttribute('data-industry-name') || card.textContent || '').toLowerCase();
                if (query === '' || name.includes(query)) {
                    card.style.display = 'flex';
                    visibleCount++;
                } else {
                    card.style.display = 'none';
                }
            });

            if (emptyMsg) {
                emptyMsg.style.display = visibleCount === 0 ? 'block' : 'none';
            }
        });
    }

    // Helper: Universal Placeholder Typewriter Rotator
    function createPlaceholderRotator(inputId, placeholders, defaultPlaceholder) {
        const input = document.getElementById(inputId);
        if (!input || !placeholders || placeholders.length === 0) return;

        let currentIdx = 0;
        let charIdx = 0;
        let isDeleting = false;
        let timer = null;
        let isFocused = false;

        function typeEffect() {
            if (isFocused) return;

            const fullText = placeholders[currentIdx];

            if (isDeleting) {
                charIdx--;
            } else {
                charIdx++;
            }

            input.placeholder = fullText.substring(0, charIdx);

            let typeSpeed = isDeleting ? 25 : 55;

            if (!isDeleting && charIdx === fullText.length) {
                typeSpeed = 1800; // Pause on complete phrase
                isDeleting = true;
            } else if (isDeleting && charIdx === 0) {
                isDeleting = false;
                currentIdx = (currentIdx + 1) % placeholders.length;
                typeSpeed = 400; // Pause before next phrase
            }

            timer = setTimeout(typeEffect, typeSpeed);
        }

        input.addEventListener('focus', () => {
            isFocused = true;
            if (timer) clearTimeout(timer);
            input.placeholder = defaultPlaceholder || "Search...";
        });

        input.addEventListener('blur', () => {
            isFocused = false;
            if (!input.value) {
                charIdx = 0;
                isDeleting = false;
                typeEffect();
            }
        });

        typeEffect();
    }

    function initAllDirectoryPlaceholderRotators() {
        createPlaceholderRotator('company-search', [
            "Barclays...",
            "Deloitte...",
            "PwC...",
            "Google UK...",
            "Amazon UK...",
            "Reed Recruitment...",
            "NHS Trust..."
        ], "Company name (e.g. Barclays, Google)...");

        createPlaceholderRotator('role-search-input', [
            "Software Developer...",
            "Registered Nurse...",
            "Qualified Accountant...",
            "Project Manager...",
            "Data Analyst...",
            "Marketing Specialist...",
            "Electrician...",
            "Teacher..."
        ], "Role name (e.g. Developer, Nurse)...");

        createPlaceholderRotator('salary-search-input', [
            "Software Engineer...",
            "NHS Nurse...",
            "Project Manager...",
            "Accountant...",
            "Data Scientist...",
            "Site Manager...",
            "HR Manager..."
        ], "Role name (e.g. Developer, Nurse)...");

        createPlaceholderRotator('location-search-input', [
            "London...",
            "Manchester...",
            "Birmingham...",
            "Leeds...",
            "Glasgow...",
            "Edinburgh...",
            "Bristol...",
            "Liverpool..."
        ], "City name (e.g. London, Manchester)...");

        createPlaceholderRotator('industry-search-input', [
            "Technology & IT...",
            "Healthcare & Nursing...",
            "Finance & Accountancy...",
            "Education & Teaching...",
            "Construction & Trades...",
            "Retail & Hospitality...",
            "Engineering & Manufacturing..."
        ], "Industry name (e.g. Technology)...");
    }

    // Desktop Search Bar Sticky On Scroll Controller
    function initDesktopStickySearchBar() {
        const searchBars = document.querySelectorAll('.search-bar');
        if (!searchBars.length) return;

        searchBars.forEach(searchBar => {
            if (searchBar.closest('.footer') || searchBar.closest('.mobile-menu')) return;

            let wrapper = searchBar.parentElement;
            if (!wrapper || !wrapper.classList.contains('search-bar-wrapper')) {
                wrapper = document.createElement('div');
                wrapper.className = 'search-bar-wrapper';
                searchBar.parentNode.insertBefore(wrapper, searchBar);
                wrapper.appendChild(searchBar);
            }

            const siteHeader = document.getElementById('site-header');

            function updateStickyState() {
                if (window.innerWidth <= 768) {
                    if (searchBar.classList.contains('search-bar--sticky')) {
                        searchBar.classList.remove('search-bar--sticky');
                        searchBar.style.top = '';
                        wrapper.style.minHeight = '';
                    }
                    return;
                }

                const headerHeight = siteHeader ? siteHeader.offsetHeight : 70;
                const wrapperRect = wrapper.getBoundingClientRect();
                const shouldBeSticky = wrapperRect.top <= (headerHeight + 10);

                if (shouldBeSticky) {
                    if (!searchBar.classList.contains('search-bar--sticky')) {
                        wrapper.style.minHeight = searchBar.offsetHeight + 'px';
                        searchBar.classList.add('search-bar--sticky');
                    }
                    const headerCurrentBottom = siteHeader ? (siteHeader.getBoundingClientRect().bottom + 8) : 78;
                    searchBar.style.top = Math.max(headerCurrentBottom, 12) + 'px';
                } else {
                    if (searchBar.classList.contains('search-bar--sticky')) {
                        searchBar.classList.remove('search-bar--sticky');
                        searchBar.style.top = '';
                        wrapper.style.minHeight = '';
                    }
                }
            }

            window.addEventListener('scroll', updateStickyState, { passive: true });
            window.addEventListener('resize', updateStickyState, { passive: true });
            updateStickyState();
        });
    }

    // Initialize all directory & view controllers
    initDesktopStickySearchBar();
    initViewModeController();
    initFooterBarObserver();
    initInfiniteRolesObserver();
    initInfiniteSalariesObserver();
    initStickyHeaders();
    initRoleSearchInstant();
    initSalarySearchInstant();
    initLocationSearchInstant();
    initIndustrySearchInstant();
    initAllDirectoryPlaceholderRotators();
});

/* =========================================================
   Next-Level Global Modal Alert & Confirmation System
   ========================================================= */
(function() {
    const ICONS = {
        success: `<svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="#10b981" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path><polyline points="22 4 12 14.01 9 11.01"></polyline></svg>`,
        info: `<svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="#0A66C2" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg>`,
        warning: `<svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="#f59e0b" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path><line x1="12" y1="9" x2="12" y2="13"></line><line x1="12" y1="17" x2="12.01" y2="17"></line></svg>`,
        error: `<svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="#ef4444" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"></circle><line x1="15" y1="9" x2="9" y2="15"></line><line x1="9" y1="9" x2="15" y2="15"></line></svg>`
    };

    function ensureModalContainer() {
        let container = document.getElementById('JobyUK-modal-system');
        if (!container) {
            container = document.createElement('div');
            container.id = 'JobyUK-modal-system';
            document.body.appendChild(container);
        }
        return container;
    }

    function escapeModalHtml(str) {
        if (!str) return '';
        const d = document.createElement('div');
        d.textContent = str;
        return d.innerHTML;
    }

    function showJobyUKModal(opts) {
        const title = opts.title || '';
        const message = opts.message || '';
        const type = opts.type || 'info';
        const confirmText = opts.confirmText || 'Got it';
        const cancelText = opts.cancelText || null;
        const onConfirm = opts.onConfirm || null;
        const onCancel = opts.onCancel || null;

        const container = ensureModalContainer();
        const iconSvg = ICONS[type] || ICONS.info;

        const overlay = document.createElement('div');
        overlay.className = 'h-modal-overlay';
        overlay.innerHTML = `
            <div class="h-modal-backdrop"></div>
            <div class="h-modal-card" role="dialog" aria-modal="true">
                <div class="h-modal-header">
                    <div class="h-modal-icon-wrap h-modal-icon-wrap--${type}">
                        ${iconSvg}
                    </div>
                    <button type="button" class="h-modal-close-btn" aria-label="Close">
                        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>
                    </button>
                </div>
                <div class="h-modal-body">
                    ${title ? `<h3 class="h-modal-title">${escapeModalHtml(title)}</h3>` : ''}
                    <p class="h-modal-message">${escapeModalHtml(message)}</p>
                </div>
                <div class="h-modal-footer">
                    ${cancelText ? `<button type="button" class="h-modal-btn h-modal-btn--cancel">${escapeModalHtml(cancelText)}</button>` : ''}
                    <button type="button" class="h-modal-btn h-modal-btn--confirm h-modal-btn--${type}">${escapeModalHtml(confirmText)}</button>
                </div>
            </div>
        `;

        container.appendChild(overlay);
        requestAnimationFrame(() => {
            overlay.classList.add('h-modal-overlay--open');
        });

        const close = () => {
            overlay.classList.remove('h-modal-overlay--open');
            setTimeout(() => {
                overlay.remove();
            }, 220);
        };

        const backdrop = overlay.querySelector('.h-modal-backdrop');
        const closeBtn = overlay.querySelector('.h-modal-close-btn');
        const confirmBtn = overlay.querySelector('.h-modal-btn--confirm');
        const cancelBtn = overlay.querySelector('.h-modal-btn--cancel');

        if (backdrop) backdrop.addEventListener('click', () => { close(); if (onCancel) onCancel(); });
        if (closeBtn) closeBtn.addEventListener('click', () => { close(); if (onCancel) onCancel(); });
        if (cancelBtn) cancelBtn.addEventListener('click', () => { close(); if (onCancel) onCancel(); });

        if (confirmBtn) {
            confirmBtn.focus();
            confirmBtn.addEventListener('click', () => {
                close();
                if (onConfirm) onConfirm();
            });
        }

        const handleKey = (e) => {
            if (e.key === 'Escape') {
                close();
                if (onCancel) onCancel();
                document.removeEventListener('keydown', handleKey);
            }
        };
        document.addEventListener('keydown', handleKey);
    }

    function showJobyUKToast(message, type = 'info', duration = 3000) {
        let toastBox = document.getElementById('JobyUK-toast-container');
        if (!toastBox) {
            toastBox = document.createElement('div');
            toastBox.id = 'JobyUK-toast-container';
            toastBox.className = 'h-toast-container';
            document.body.appendChild(toastBox);
        }

        const iconSvg = ICONS[type] || ICONS.info;
        const toast = document.createElement('div');
        toast.className = `h-toast-item h-toast-item--${type}`;
        toast.innerHTML = `
            <div class="h-toast-icon">${iconSvg}</div>
            <div class="h-toast-text">${escapeModalHtml(message)}</div>
        `;

        toastBox.appendChild(toast);
        requestAnimationFrame(() => {
            toast.classList.add('h-toast-item--open');
        });

        setTimeout(() => {
            toast.classList.remove('h-toast-item--open');
            setTimeout(() => toast.remove(), 250);
        }, duration);
    }

    // Expose Global APIs
    window.JobyUKAlert = function(msgOrObj) {
        if (typeof msgOrObj === 'string') {
            showJobyUKModal({ message: msgOrObj, type: 'info' });
        } else {
            showJobyUKModal(msgOrObj);
        }
    };

    window.JobyUKConfirm = function(msgOrObj, onConfirm, onCancel) {
        if (typeof msgOrObj === 'string') {
            showJobyUKModal({
                message: msgOrObj,
                type: 'warning',
                cancelText: 'Cancel',
                confirmText: 'Confirm',
                onConfirm: onConfirm,
                onCancel: onCancel
            });
        } else {
            showJobyUKModal({
                cancelText: 'Cancel',
                confirmText: 'Confirm',
                ...msgOrObj
            });
        }
    };

    window.JobyUKToast = showJobyUKToast;

    // Elegant override for default browser alert so that no ugly alert ever fires!
    window.alert = function(message) {
        showJobyUKToast(message, 'info', 3200);
    };
})();

// ================================================================
// Anti-Freeze & Scroll Protection Watchdog
// Prevents Google Vignette / Interstitial ad tags from trapping
// <body> with position: fixed; top: -...px; overflow: hidden;
// and ensures smooth scrolling and sticky header functionality.
// ================================================================
(function() {
    let isReleasing = false;

    function isSiteModalActive() {
        const drawer = document.querySelector('.mobile-drawer-overlay--open, .jy-drawer-open, #jMobileDrawer.is-open');
        const search = document.querySelector('.gsm-overlay--open');
        const wa = document.body && document.body.classList.contains('wa-modal-open');
        const report = document.getElementById('report-job-modal');
        const reportOpen = report && report.style.display !== 'none' && report.style.display !== '';
        const JobyUKModal = document.querySelector('.h-modal-overlay');
        return !!(drawer || search || wa || reportOpen || JobyUKModal);
    }

    function releaseScrollTrap() {
        if (isReleasing) return;

        // Strip #google_vignette hash if interstitial failed or left behind
        if (window.location.hash === '#google_vignette') {
            try {
                history.replaceState(null, document.title, window.location.pathname + window.location.search);
            } catch (e) {}
        }

        if (!document.body || isSiteModalActive()) return;

        isReleasing = true;
        try {
            if (document.body.hasAttribute('aria-hidden')) {
                document.body.removeAttribute('aria-hidden');
            }
            if (document.body.style.position === 'fixed') {
                const prevTop = parseInt(document.body.style.top || '0', 10);
                document.body.style.position = '';
                document.body.style.top = '';
                document.body.style.overflow = '';
                if (prevTop < 0) {
                    window.scrollTo(0, Math.abs(prevTop));
                }
            } else if (document.body.style.overflow === 'hidden') {
                document.body.style.overflow = '';
            }
        } finally {
            setTimeout(function() { isReleasing = false; }, 60);
        }
    }

    function sanitizeAdStyles() {
        if (!document.head) return;
        try {
            const styles = document.head.querySelectorAll('style');
            for (let i = 0; i < styles.length; i++) {
                const s = styles[i];
                if (s.textContent && (s.textContent.indexOf('overflow-x:hidden!important') !== -1 || s.textContent.indexOf('overflow-x: hidden') !== -1)) {
                    s.textContent = s.textContent.replace(/html\s*,\s*body\s*\{[^}]*overflow-x:[^}]+}/gi, '/* wfa-body-scroll-sanitized */');
                }
            }
        } catch (e) {}
    }

    // Run on critical life-cycle events
    ['DOMContentLoaded', 'load', 'pageshow', 'popstate', 'hashchange'].forEach(function(evt) {
        window.addEventListener(evt, function() {
            sanitizeAdStyles();
            releaseScrollTrap();
        }, { passive: true });
    });

    // Active MutationObserver on <body> and <head>
    if (typeof MutationObserver !== 'undefined') {
        const observer = new MutationObserver(function() {
            if (isReleasing || isSiteModalActive()) return;
            if (document.body && (document.body.style.position === 'fixed' || document.body.getAttribute('aria-hidden') === 'true')) {
                releaseScrollTrap();
            }
        });

        const headObserver = new MutationObserver(function(mutations) {
            for (let i = 0; i < mutations.length; i++) {
                const added = mutations[i].addedNodes;
                for (let j = 0; j < added.length; j++) {
                    const node = added[j];
                    if (node && node.tagName === 'STYLE' && node.textContent && (node.textContent.indexOf('overflow-x:hidden!important') !== -1 || node.textContent.indexOf('overflow-x: hidden') !== -1)) {
                        node.textContent = node.textContent.replace(/html\s*,\s*body\s*\{[^}]*overflow-x:[^}]+}/gi, '/* wfa-body-scroll-sanitized */');
                    }
                }
            }
        });

        const initObserver = function() {
            if (document.body) {
                observer.observe(document.body, { attributes: true, attributeFilter: ['style', 'aria-hidden'] });
            }
            if (document.head) {
                headObserver.observe(document.head, { childList: true, subtree: true });
                sanitizeAdStyles();
            }
        };

        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', initObserver);
        } else {
            initObserver();
        }
    }
})();

// =========================================================================
// JobyUK 100% Dynamic Client Saved Jobs System (LocalStorage + Live Sync)
// =========================================================================
window.JobyUKSaved = (function() {
    const STORAGE_KEY = 'jobyuk_saved_jobs_v1';

    function getStorage() {
        try {
            const raw = localStorage.getItem(STORAGE_KEY);
            return raw ? JSON.parse(raw) : [];
        } catch (e) {
            console.warn('JobyUKSaved: LocalStorage unavailable', e);
            return [];
        }
    }

    function setStorage(arr) {
        try {
            localStorage.setItem(STORAGE_KEY, JSON.stringify(arr));
        } catch (e) {
            console.warn('JobyUKSaved: Failed to write localStorage', e);
        }
    }

    function getAll() {
        return getStorage();
    }

    function has(id) {
        if (!id) return false;
        const list = getStorage();
        const strId = String(id).trim();
        return list.some(item => String(item.id).trim() === strId);
    }

    function add(job) {
        if (!job || !job.id) return false;
        const list = getStorage();
        const strId = String(job.id).trim();
        if (list.some(item => String(item.id).trim() === strId)) return false;
        
        const cleanJob = {
            id: strId,
            title: job.title || 'Job Opportunity',
            slug: job.slug || '',
            employer: job.employer || 'Verified Employer',
            employerSlug: job.employerSlug || '',
            logoUrl: job.logoUrl || '',
            location: job.location || 'United Kingdom',
            isRemote: !!job.isRemote,
            salary: job.salary || 'Competitive salary',
            timeAgo: job.timeAgo || 'Recently',
            savedAt: Date.now()
        };
        list.unshift(cleanJob);
        setStorage(list);
        syncBadges();
        syncButtons();
        showToast('Saved to your bookmarks! 🔖');
        window.dispatchEvent(new CustomEvent('jobyuk:saved-changed', { detail: { action: 'add', job: cleanJob } }));
        return true;
    }

    function remove(id) {
        if (!id) return false;
        const list = getStorage();
        const strId = String(id).trim();
        const filtered = list.filter(item => String(item.id).trim() !== strId);
        if (filtered.length === list.length) return false;
        setStorage(filtered);
        syncBadges();
        syncButtons();
        showToast('Job removed from saved list.');
        window.dispatchEvent(new CustomEvent('jobyuk:saved-changed', { detail: { action: 'remove', id: strId } }));
        return true;
    }

    function toggle(job) {
        if (!job || !job.id) return false;
        if (has(job.id)) {
            remove(job.id);
            return false;
        } else {
            add(job);
            return true;
        }
    }

    function clear() {
        setStorage([]);
        syncBadges();
        syncButtons();
        showToast('All saved jobs cleared.');
        window.dispatchEvent(new CustomEvent('jobyuk:saved-changed', { detail: { action: 'clear' } }));
    }

    function count() {
        return getStorage().length;
    }

    function syncBadges() {
        const c = count();
        const desktopBadge = document.getElementById('navSavedJobsBadge');
        const mobileBadge = document.getElementById('navSavedJobsBadgeMobile');
        
        [desktopBadge, mobileBadge].forEach(b => {
            if (!b) return;
            if (c > 0) {
                b.textContent = c > 99 ? '99+' : c;
                b.style.display = 'inline-flex';
            } else {
                b.style.display = 'none';
            }
        });
    }

    function syncButtons() {
        const list = getStorage();
        const savedIds = new Set(list.map(item => String(item.id).trim()));

        // Update all card bookmark buttons
        document.querySelectorAll('.jy-dream-bookmark-btn').forEach(btn => {
            const jid = (btn.getAttribute('data-job-id') || '').trim();
            if (jid) {
                if (savedIds.has(jid)) {
                    btn.classList.add('saved');
                    btn.classList.add('active');
                    btn.setAttribute('aria-label', 'Remove from saved jobs');
                    btn.setAttribute('title', 'Saved');
                } else {
                    btn.classList.remove('saved');
                    btn.classList.remove('active');
                    btn.setAttribute('aria-label', 'Save job');
                    btn.setAttribute('title', 'Save job');
                }
            }
        });

        // Update detail page save buttons
        document.querySelectorAll('.jy-detail-save-btn, .jy-sticky-apply-bar__save-btn').forEach(btn => {
            const jid = (btn.getAttribute('data-job-id') || '').trim();
            if (jid) {
                const isSaved = savedIds.has(jid);
                if (isSaved) {
                    btn.classList.add('saved');
                    const span = btn.querySelector('span');
                    if (span) span.textContent = 'Saved';
                } else {
                    btn.classList.remove('saved');
                    const span = btn.querySelector('span');
                    if (span) span.textContent = 'Save';
                }
            }
        });
    }

    function toggleFromButton(btn, event) {
        if (event) {
            event.stopPropagation();
            event.preventDefault();
        }
        if (!btn) return;

        const job = {
            id: btn.getAttribute('data-job-id'),
            title: btn.getAttribute('data-job-title'),
            slug: btn.getAttribute('data-job-slug'),
            employer: btn.getAttribute('data-job-employer'),
            employerSlug: btn.getAttribute('data-job-employer-slug'),
            logoUrl: btn.getAttribute('data-job-logo'),
            location: btn.getAttribute('data-job-location'),
            isRemote: btn.getAttribute('data-job-remote') === 'true',
            salary: btn.getAttribute('data-job-salary'),
            timeAgo: btn.getAttribute('data-job-time')
        };

        if (!job.id) {
            const card = btn.closest('[data-job-id]');
            if (card) {
                job.id = card.getAttribute('data-job-id');
            }
        }

        if (!job.id) return;
        toggle(job);
    }

    function showToast(msg) {
        let toast = document.getElementById('jobyukGlobalToast');
        if (!toast) {
            toast = document.createElement('div');
            toast.id = 'jobyukGlobalToast';
            toast.style.cssText = 'position:fixed;bottom:24px;left:50%;transform:translateX(-50%) translateY(100px);background:#0B1F1A;color:#FFFFFF;padding:12px 24px;border-radius:9999px;font-size:14px;font-weight:700;box-shadow:0 10px 25px rgba(0,0,0,0.25);z-index:999999;display:flex;align-items:center;gap:10px;transition:transform 0.3s cubic-bezier(0.16,1,0.3,1),opacity 0.3s ease;opacity:0;pointer-events:none;border:1.5px solid rgba(255,255,255,0.15);font-family:inherit;';
            document.body.appendChild(toast);
        }
        toast.textContent = msg;
        toast.style.opacity = '1';
        toast.style.transform = 'translateX(-50%) translateY(0)';

        if (window._jobyukToastTimer) clearTimeout(window._jobyukToastTimer);
        window._jobyukToastTimer = setTimeout(() => {
            toast.style.opacity = '0';
            toast.style.transform = 'translateX(-50%) translateY(100px)';
        }, 2600);
    }

    // Auto-init on load
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => {
            syncBadges();
            syncButtons();
        });
    } else {
        syncBadges();
        syncButtons();
    }

    // Listen to storage changes from other tabs
    window.addEventListener('storage', (e) => {
        if (e.key === STORAGE_KEY) {
            syncBadges();
            syncButtons();
            window.dispatchEvent(new CustomEvent('jobyuk:saved-changed', { detail: { action: 'storage' } }));
        }
    });

    return {
        getAll,
        has,
        add,
        remove,
        toggle,
        clear,
        count,
        syncBadges,
        syncButtons,
        toggleFromButton,
        showToast
    };
})();



