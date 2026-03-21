// static/script.js

let currentLang = 'hr';
let currentGallery = 'main';
let imagesData = [];
let mainGalleryImages = [];
let lastScrollPosition = 0;
let sourceImageId = null;

const translations = {
    hr: {
        heroTitle: "Zavičajna udruga Magriž Martinščića",
        heroDesc: "Promicanje vrijednosti i održivi razvoj Martinšćice i Cresa.",
        scrollMore: "Vidi više",
        loader: "Učitavanje...",
        navHeader: "Navigacija",
        linkHome: "Početna",
        imgLinkPrefix: "Slika "
    },
    en: {
        heroTitle: "Homeland association Magriž Martinščića",
        heroDesc: "Promoting values. Advancing sustainable development of Martinšćica and Cres.",
        scrollMore: "See more",
        loader: "Loading...",
        navHeader: "Navigation",
        linkHome: "Home",
        imgLinkPrefix: "Image "
    },
    it: {
        heroTitle: "Associazone Della Patria Magriž Martinščića",
        heroDesc: "Promuovere i valori. Sviluppo sostenibile di Martinšćica e Cres.",
        scrollMore: "Vedi altro",
        loader: "Caricamento...",
        navHeader: "Navigazione",
        linkHome: "Inizio",
        imgLinkPrefix: "Immagine "
    },
    de: {
        heroTitle: "Heimatverband Magriž Martinščića",
        heroDesc: "Werte fördern. Nachhaltige Entwicklung von Martinšćica und Cres stärken.",
        scrollMore: "Mehr sehen",
        loader: "Laden...",
        navHeader: "Navigation",
        linkHome: "Startseite",
        imgLinkPrefix: "Bild "
    }
};

document.addEventListener('DOMContentLoaded', () => {
    fetchImages('main');
    updateUIText();
});

async function fetchImages(galleryCode, targetImgId = null) {
    currentGallery = galleryCode;
    const mainFeed = document.getElementById('main-feed');
    const hero = document.getElementById('hero');
    const feedContainer = document.getElementById('feed-container');
    const backNav = document.getElementById('gallery-back-nav');

    if (galleryCode === 'main') {
        mainFeed.classList.remove('is-gallery');
        hero.style.display = 'flex';
        feedContainer.classList.remove('grid-mode');
        if (backNav) backNav.style.display = 'none';
    } else {
        mainFeed.classList.add('is-gallery');
        hero.style.display = 'none';
        feedContainer.classList.add('grid-mode');
        if (backNav) backNav.style.display = 'flex';
    }

    feedContainer.innerHTML = `<div id="loader" class="loader">${translations[currentLang].loader}</div>`;

    try {
        const response = await fetch(`/api/images?gallery=${galleryCode}`);
        if (!response.ok) throw new Error('Network response was not ok');
        imagesData = await response.json();
        
        if (galleryCode === 'main' && mainGalleryImages.length === 0) {
            mainGalleryImages = imagesData;
            populateSidebar();
        }

        feedContainer.innerHTML = ''; 
        renderImages();
        
        if (galleryCode !== 'main') window.scrollTo(0, 0);
        
        if (targetImgId) {
            const img = imagesData.find(i => i.id === targetImgId);
            if (img) openDetail(img, false);
        }
    } catch (error) {
        console.error('Fetch error:', error);
        feedContainer.innerHTML = '<p>Error loading content.</p>';
    }
}

function populateSidebar() {
    const navList = document.getElementById('nav-list');
    navList.innerHTML = `<li><a href="#hero" id="link-home" onclick="goHome(event)">${translations[currentLang].linkHome}</a></li><li class="nav-divider"></li>`;
    
    mainGalleryImages.forEach((item) => {
        const cardId = `media-card-${item.type}-${item.id}`;
        const li = document.createElement('li');
        const a = document.createElement('a');
        a.href = `#${cardId}`;
        a.className = 'img-nav-link';
        a.textContent = getDescription(item, currentLang, false);
        
        a.addEventListener('click', (e) => {
            e.preventDefault();
            if (currentGallery !== 'main') {
                fetchImages('main').then(() => {
                    setTimeout(() => scrollToImage(cardId), 100);
                });
            } else {
                closeDetail();
                closeModal();
                scrollToImage(cardId);
            }
        });
        li.appendChild(a);
        navList.appendChild(li);
    });
}

function scrollToImage(id) {
    const el = document.getElementById(id);
    if (el) el.scrollIntoView({ behavior: 'smooth' });
}

function setLanguage(lang) {
    currentLang = lang;
    document.documentElement.lang = lang;
    updateUIText();
    populateSidebar();

    // Update existing cards
    document.querySelectorAll('.image-card').forEach(card => {
        const id = parseInt(card.dataset.id);
        const item = imagesData.find(i => i.id === id);
        if (item) card.querySelector('.image-desc').textContent = getDescription(item, currentLang, false);
    });

    // Update detail view if open
    const detailView = document.getElementById('detail-view');
    if (detailView.style.display === 'block') {
        const id = parseInt(detailView.dataset.currentId);
        const item = imagesData.find(i => i.id === id);
        if (item) openDetail(item, false);
    }

    // Update modal if open
    const modal = document.getElementById('image-modal');
    if (modal.style.display === 'block') {
        const id = parseInt(modal.dataset.currentId);
        const item = imagesData.find(i => i.id === id);
        if (item) document.getElementById('modal-desc').textContent = getDescription(item, currentLang, true);
    }
}

function updateUIText() {
    const t = translations[currentLang];
    const heroTitle = document.getElementById('hero-title');
    if (heroTitle) heroTitle.textContent = t.heroTitle;
    const heroDesc = document.getElementById('hero-description');
    if (heroDesc) heroDesc.textContent = t.heroDesc;
    const navHeader = document.getElementById('nav-header');
    if (navHeader) navHeader.textContent = t.navHeader;
    
    // Back buttons are icon-only now, so no text update needed
}

function getDescription(item, lang, long = false) {
    const prefix = long ? 'long_desc_' : 'desc_';
    return item[prefix + lang] || item[prefix + 'hr'] || "";
}

function renderImages() {
    const container = document.getElementById('feed-container');
    imagesData.forEach((item) => {
        const cardId = `media-card-${item.type}-${item.id}`;
        const card = document.createElement('div');
        card.className = 'image-card' + (item.type === 'video' ? ' video-card' : '');
        card.id = cardId;
        card.dataset.id = item.id;

        const mediaWrapper = document.createElement('div');
        mediaWrapper.className = 'media-wrapper-inner';

        if (item.type === 'video') {
            const thumb = document.createElement('img');
            thumb.src = item.thumbnail_url || 'https://placehold.co/800x600?text=Video';
            mediaWrapper.appendChild(thumb);
            
            const playBtn = document.createElement('div');
            playBtn.className = 'play-overlay';
            playBtn.innerHTML = '▶';
            mediaWrapper.appendChild(playBtn);
        } else {
            const imgTag = document.createElement('img');
            imgTag.src = item.url;
            imgTag.alt = 'Muzej Magriž';
            imgTag.loading = 'lazy';
            mediaWrapper.appendChild(imgTag);
        }

        const desc = document.createElement('div');
        desc.className = 'image-desc';
        desc.textContent = getDescription(item, currentLang, false);

        card.appendChild(mediaWrapper);
        card.appendChild(desc);
        
        card.addEventListener('click', () => {
            if (item.type === 'video') {
                playVideo(item);
            } else {
                if (currentGallery === 'main') openDetail(item, true);
                else openModal(item);
            }
        });
        
        container.appendChild(card);
        observer.observe(card);
    });
}

function playVideo(item) {
    const video = document.createElement('video');
    video.src = item.url;
    video.controls = true;
    video.autoplay = true;
    video.style.width = '100%';
    video.style.height = '100%';
    video.style.backgroundColor = 'black';

    if (video.requestFullscreen) {
        document.body.appendChild(video);
        video.requestFullscreen();
    } else if (video.webkitRequestFullscreen) {
        document.body.appendChild(video);
        video.webkitRequestFullscreen();
    }

    video.onfullscreenchange = () => {
        if (!document.fullscreenElement) {
            video.pause();
            video.remove();
        }
    };
    
    video.onerror = () => {
        alert("Error loading video");
        video.remove();
    };
}

function openDetail(item, saveScroll = true) {
    if (saveScroll) lastScrollPosition = window.scrollY;
    
    document.getElementById('main-feed').style.display = 'none';
    const detailView = document.getElementById('detail-view');
    detailView.style.display = 'block';
    detailView.dataset.currentId = item.id;
    detailView.style.backgroundImage = `url('${item.url}')`;

    const descContainer = document.getElementById('detail-desc');
    if (descContainer) {
        descContainer.innerHTML = getDescription(item, currentLang, true);

        if (item.links && item.links.length > 0) {
            const linksDiv = document.createElement('div');
            linksDiv.className = 'related-links';
            item.links.forEach(link => {
                const btn = document.createElement('button');
                btn.className = 'gallery-link-btn';
                btn.textContent = link['name_' + currentLang] || link.name_hr;
                btn.onclick = () => {
                    sourceImageId = item.id;
                    closeDetail();
                    fetchImages(link.target_gallery_code);
                };
                linksDiv.appendChild(btn);
            });
            descContainer.appendChild(linksDiv);
        }
    }
    if (saveScroll) detailView.scrollTo(0, 0);
}

function closeDetail() {
    document.getElementById('main-feed').style.display = 'block';
    document.getElementById('detail-view').style.display = 'none';
    if (lastScrollPosition > 0) window.scrollTo(0, lastScrollPosition);
}

function openModal(item) {
    const modal = document.getElementById('image-modal');
    modal.style.display = 'block';
    modal.dataset.currentId = item.id;
    document.getElementById('modal-img').src = item.url;
    document.getElementById('modal-desc').textContent = getDescription(item, currentLang, true);
    document.body.style.overflow = 'hidden';
}

function closeModal() {
    document.getElementById('image-modal').style.display = 'none';
    document.body.style.overflow = 'auto';
}

function goHome(event) {
    if (event) event.preventDefault();
    closeDetail();
    closeModal();
    if (currentGallery !== 'main') {
        const targetId = sourceImageId;
        sourceImageId = null;
        fetchImages('main', targetId);
    } else {
        window.scrollTo({ top: 0, behavior: 'smooth' });
    }
}

const observerOptions = { root: null, rootMargin: '0px', threshold: 0.1 };
const observer = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        if (entry.isIntersecting) {
            entry.target.style.opacity = '1';
            entry.target.style.transform = 'translateY(0)';
        }
    });
}, observerOptions);
