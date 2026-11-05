/**
 * Escocar Autos - Main Application Scripts
 */
document.addEventListener('DOMContentLoaded', () => {
  'use strict';

  // 1. Vehicle Detail Gallery & Floating Lightbox
  const mainImg = document.getElementById('mainGalleryImg');
  const thumbs = document.querySelectorAll('.thumb-item');
  const galleryPrevBtn = document.getElementById('galleryPrevBtn');
  const galleryNextBtn = document.getElementById('galleryNextBtn');
  const galleryCounterNum = document.getElementById('galleryCurrentNum');
  const galleryWrapper = document.getElementById('mainGalleryWrapper');
  const galleryExpandBtn = document.getElementById('galleryExpandBtn');

  let currentGalleryIdx = 0;

  function setGalleryImage(idx) {
    if (!thumbs || thumbs.length === 0) return;
    if (idx < 0) idx = thumbs.length - 1;
    if (idx >= thumbs.length) idx = 0;
    currentGalleryIdx = idx;

    thumbs.forEach((t, i) => {
      t.classList.toggle('active', i === idx);
    });

    const selectedThumb = thumbs[idx];
    if (selectedThumb && mainImg) {
      const newSrc = selectedThumb.getAttribute('data-img');
      if (newSrc && mainImg.src !== newSrc) {
        mainImg.style.opacity = '0.4';
        mainImg.src = newSrc;
        mainImg.onload = () => { mainImg.style.opacity = '1'; };
        setTimeout(() => { mainImg.style.opacity = '1'; }, 200);
      }
      selectedThumb.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'center' });
    }

    if (galleryCounterNum) {
      galleryCounterNum.textContent = idx + 1;
    }
  }

  if (thumbs.length > 0) {
    thumbs.forEach((thumb, i) => {
      thumb.addEventListener('click', () => {
        setGalleryImage(i);
      });
    });

    if (galleryPrevBtn) {
      galleryPrevBtn.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        setGalleryImage(currentGalleryIdx - 1);
      });
    }

    if (galleryNextBtn) {
      galleryNextBtn.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        setGalleryImage(currentGalleryIdx + 1);
      });
    }

    // Touch swipe support on main gallery image
    if (galleryWrapper) {
      let touchStartX = 0;
      let touchEndX = 0;

      galleryWrapper.addEventListener('touchstart', (e) => {
        touchStartX = e.changedTouches[0].screenX;
      }, { passive: true });

      galleryWrapper.addEventListener('touchend', (e) => {
        touchEndX = e.changedTouches[0].screenX;
        const diffX = touchEndX - touchStartX;
        if (Math.abs(diffX) > 40) {
          if (diffX > 0) {
            setGalleryImage(currentGalleryIdx - 1);
          } else {
            setGalleryImage(currentGalleryIdx + 1);
          }
        }
      }, { passive: true });
    }
  }

  function getDetailCarImages() {
    if (galleryWrapper) {
      try {
        const raw = galleryWrapper.getAttribute('data-images') || '[]';
        const parsed = JSON.parse(raw);
        if (Array.isArray(parsed) && parsed.length > 0) return parsed;
      } catch (e) {}
    }
    if (thumbs && thumbs.length > 0) {
      const arr = [];
      thumbs.forEach(t => {
        const src = t.getAttribute('data-img');
        if (src) arr.push(src);
      });
      if (arr.length > 0) return arr;
    }
    if (mainImg && mainImg.src) return [mainImg.src];
    return [];
  }

  // =========================================================================
  // Floating Lightbox Gallery with Backdrop Blur & Pan/Zoom
  // =========================================================================
  const lightboxModal = document.getElementById('imageLightboxModal');
  const lightboxBackdrop = document.getElementById('lightboxBackdrop');
  const lightboxCloseBtn = document.getElementById('lightboxCloseBtn');
  const lightboxTitle = document.getElementById('lightboxTitle');
  const lightboxCounter = document.getElementById('lightboxCounter');
  const lightboxQualityBadge = document.getElementById('lightboxQualityBadge');
  const lightboxImg = document.getElementById('lightboxImg');
  const lightboxTransform = document.getElementById('lightboxTransformContainer');
  const lightboxPrevBtn = document.getElementById('lightboxPrevBtn');
  const lightboxNextBtn = document.getElementById('lightboxNextBtn');
  const lightboxZoomInBtn = document.getElementById('lightboxZoomInBtn');
  const lightboxZoomOutBtn = document.getElementById('lightboxZoomOutBtn');
  const lightboxResetZoomBtn = document.getElementById('lightboxResetZoomBtn');
  const lightboxZoomLevel = document.getElementById('lightboxZoomLevel');
  const lightboxThumbsBar = document.getElementById('lightboxThumbsBar');
  const lightboxThumbsInner = document.getElementById('lightboxThumbsInner');
  const lightboxViewport = document.getElementById('lightboxViewport');

  let lbImages = [];
  let lbCurrentIdx = 0;
  let lbScale = 1;
  let lbMaxScale = 3.5;
  let lbTranslateX = 0;
  let lbTranslateY = 0;
  let lbIsDragging = false;
  let lbDragStartX = 0;
  let lbDragStartY = 0;
  let lbDragStartTx = 0;
  let lbDragStartTy = 0;
  let lbHasMoved = false;

  function updateLightboxTransform(smooth = false) {
    if (!lightboxTransform) return;
    if (smooth) {
      lightboxTransform.classList.add('smooth');
    } else {
      lightboxTransform.classList.remove('smooth');
    }
    lightboxTransform.style.transform = `translate(${lbTranslateX}px, ${lbTranslateY}px) scale(${lbScale})`;
    
    if (lightboxZoomLevel) {
      lightboxZoomLevel.textContent = `${Math.round(lbScale * 100)}%`;
    }

    if (lightboxImg) {
      if (lbScale > 1) {
        lightboxImg.classList.add('zoomed');
      } else {
        lightboxImg.classList.remove('zoomed', 'grabbing');
      }
    }
  }

  function resetLightboxZoom(smooth = true) {
    lbScale = 1;
    lbTranslateX = 0;
    lbTranslateY = 0;
    updateLightboxTransform(smooth);
  }

  function clampLightboxTranslation(targetScale = lbScale) {
    if (targetScale <= 1 || !lightboxImg || !lightboxViewport) {
      lbTranslateX = 0;
      lbTranslateY = 0;
      return;
    }
    const vpRect = lightboxViewport.getBoundingClientRect();
    const imgWidth = lightboxImg.offsetWidth * targetScale;
    const imgHeight = lightboxImg.offsetHeight * targetScale;

    const maxTx = Math.max(0, (imgWidth - vpRect.width) / 2 + 60);
    const maxTy = Math.max(0, (imgHeight - vpRect.height) / 2 + 60);

    lbTranslateX = Math.max(-maxTx, Math.min(maxTx, lbTranslateX));
    lbTranslateY = Math.max(-maxTy, Math.min(maxTy, lbTranslateY));
  }

  function setLightboxScale(newScale, focalX = null, focalY = null) {
    newScale = Math.max(1, Math.min(lbMaxScale, newScale));
    if (Math.abs(newScale - lbScale) < 0.01) return;

    if (newScale === 1) {
      resetLightboxZoom(true);
      return;
    }

    if (focalX !== null && focalY !== null && lightboxViewport) {
      const rect = lightboxViewport.getBoundingClientRect();
      const centerX = rect.left + rect.width / 2;
      const centerY = rect.top + rect.height / 2;
      const offsetX = focalX - centerX;
      const offsetY = focalY - centerY;
      const scaleRatio = newScale / lbScale;
      lbTranslateX = lbTranslateX - offsetX * (scaleRatio - 1) * 0.45;
      lbTranslateY = lbTranslateY - offsetY * (scaleRatio - 1) * 0.45;
    }

    lbScale = newScale;
    clampLightboxTranslation(lbScale);
    updateLightboxTransform(true);
  }

  function showLightboxImage(idx) {
    if (!lbImages || lbImages.length === 0) return;
    if (idx < 0) idx = lbImages.length - 1;
    if (idx >= lbImages.length) idx = 0;
    lbCurrentIdx = idx;

    resetLightboxZoom(false);

    if (lightboxCounter) {
      lightboxCounter.textContent = `${idx + 1} / ${lbImages.length}`;
    }

    if (lightboxImg) {
      lightboxImg.style.opacity = '0.35';
      lightboxImg.src = lbImages[idx];
      lightboxImg.onload = () => {
        lightboxImg.style.opacity = '1';
        const nw = lightboxImg.naturalWidth || 0;
        const nh = lightboxImg.naturalHeight || 0;
        if (nw >= 1400 || (nw > 0 && nw > lightboxImg.clientWidth * 1.3)) {
          if (lightboxQualityBadge) {
            lightboxQualityBadge.style.display = 'inline-block';
            lightboxQualityBadge.textContent = nw >= 1920 ? 'Ultra HD' : 'Alta Calidad';
          }
          lbMaxScale = 4.0;
        } else {
          if (lightboxQualityBadge) {
            lightboxQualityBadge.style.display = 'none';
          }
          lbMaxScale = 2.8;
        }
      };
      setTimeout(() => { if (lightboxImg) lightboxImg.style.opacity = '1'; }, 200);
    }

    if (lightboxThumbsInner) {
      const thumbEls = lightboxThumbsInner.querySelectorAll('.lightbox-thumb-item');
      thumbEls.forEach((t, i) => {
        t.classList.toggle('active', i === idx);
        if (i === idx) {
          t.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'center' });
        }
      });
    }

    if (thumbs && thumbs.length > 0) {
      setGalleryImage(idx);
    }
  }

  function openLightbox(images, startIdx = 0, title = 'Escocar') {
    if (!lightboxModal || !images || images.length === 0) return;
    lbImages = images;
    lbCurrentIdx = startIdx;

    if (lightboxTitle) {
      lightboxTitle.textContent = title;
    }

    if (lightboxThumbsInner) {
      lightboxThumbsInner.innerHTML = '';
      if (images.length > 1) {
        images.forEach((imgUrl, i) => {
          const item = document.createElement('div');
          item.className = `lightbox-thumb-item ${i === startIdx ? 'active' : ''}`;
          item.innerHTML = `<img src="${imgUrl}" alt="Foto ${i + 1}" loading="lazy">`;
          item.addEventListener('click', (e) => {
            e.stopPropagation();
            showLightboxImage(i);
          });
          lightboxThumbsInner.appendChild(item);
        });
        if (lightboxThumbsBar) lightboxThumbsBar.style.display = 'flex';
      } else {
        if (lightboxThumbsBar) lightboxThumbsBar.style.display = 'none';
      }
    }

    if (lightboxPrevBtn) lightboxPrevBtn.style.display = images.length > 1 ? 'flex' : 'none';
    if (lightboxNextBtn) lightboxNextBtn.style.display = images.length > 1 ? 'flex' : 'none';

    document.body.classList.add('lightbox-open');
    lightboxModal.classList.add('open');
    lightboxModal.setAttribute('aria-hidden', 'false');

    showLightboxImage(startIdx);
  }

  function closeLightbox() {
    if (!lightboxModal) return;
    lightboxModal.classList.remove('open');
    lightboxModal.setAttribute('aria-hidden', 'true');
    document.body.classList.remove('lightbox-open');
    resetLightboxZoom(false);
  }

  // Lightbox Interactions
  if (lightboxModal) {
    if (lightboxCloseBtn) {
      lightboxCloseBtn.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        closeLightbox();
      });
    }

    if (lightboxBackdrop) {
      lightboxBackdrop.addEventListener('click', closeLightbox);
    }

    if (lightboxPrevBtn) {
      lightboxPrevBtn.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        showLightboxImage(lbCurrentIdx - 1);
      });
    }

    if (lightboxNextBtn) {
      lightboxNextBtn.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        showLightboxImage(lbCurrentIdx + 1);
      });
    }

    if (lightboxZoomInBtn) {
      lightboxZoomInBtn.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        setLightboxScale(lbScale + 0.5);
      });
    }

    if (lightboxZoomOutBtn) {
      lightboxZoomOutBtn.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        setLightboxScale(lbScale - 0.5);
      });
    }

    if (lightboxResetZoomBtn) {
      lightboxResetZoomBtn.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        resetLightboxZoom(true);
      });
    }

    // Click / Double click to zoom on image
    if (lightboxImg) {
      lightboxImg.addEventListener('click', (e) => {
        e.stopPropagation();
        if (lbHasMoved) return;
        if (lbScale <= 1.05) {
          setLightboxScale(2.3, e.clientX, e.clientY);
        } else {
          resetLightboxZoom(true);
        }
      });
    }

    // Mouse wheel zooming
    if (lightboxViewport) {
      lightboxViewport.addEventListener('wheel', (e) => {
        e.preventDefault();
        const delta = e.deltaY < 0 ? 0.35 : -0.35;
        setLightboxScale(lbScale + delta, e.clientX, e.clientY);
      }, { passive: false });

      // Click on empty space in viewport to close when not zoomed
      lightboxViewport.addEventListener('click', (e) => {
        if (e.target === lightboxViewport || e.target === lightboxTransform) {
          if (lbScale > 1) {
            resetLightboxZoom(true);
          } else {
            closeLightbox();
          }
        }
      });

      // Mouse Drag / Panning
      lightboxViewport.addEventListener('mousedown', (e) => {
        if (e.button !== 0) return;
        if (lbScale <= 1) return;
        lbIsDragging = true;
        lbHasMoved = false;
        lbDragStartX = e.clientX;
        lbDragStartY = e.clientY;
        lbDragStartTx = lbTranslateX;
        lbDragStartTy = lbTranslateY;
        if (lightboxImg) lightboxImg.classList.add('grabbing');
      });

      window.addEventListener('mousemove', (e) => {
        if (!lbIsDragging) return;
        const dx = e.clientX - lbDragStartX;
        const dy = e.clientY - lbDragStartY;
        if (Math.abs(dx) > 4 || Math.abs(dy) > 4) {
          lbHasMoved = true;
        }
        lbTranslateX = lbDragStartTx + dx;
        lbTranslateY = lbDragStartTy + dy;
        clampLightboxTranslation(lbScale);
        updateLightboxTransform(false);
      });

      window.addEventListener('mouseup', () => {
        if (lbIsDragging) {
          lbIsDragging = false;
          if (lightboxImg) lightboxImg.classList.remove('grabbing');
        }
      });

      // Touch Pan / Pinch / Swipe
      let touchStartX = 0;
      let touchStartY = 0;
      let initialPinchDist = 0;
      let initialPinchScale = 1;

      lightboxViewport.addEventListener('touchstart', (e) => {
        if (e.touches.length === 1) {
          touchStartX = e.touches[0].clientX;
          touchStartY = e.touches[0].clientY;
          lbDragStartTx = lbTranslateX;
          lbDragStartTy = lbTranslateY;
          lbIsDragging = lbScale > 1;
          lbHasMoved = false;
        } else if (e.touches.length === 2) {
          lbIsDragging = false;
          initialPinchDist = Math.hypot(
            e.touches[0].clientX - e.touches[1].clientX,
            e.touches[0].clientY - e.touches[1].clientY
          );
          initialPinchScale = lbScale;
        }
      }, { passive: true });

      lightboxViewport.addEventListener('touchmove', (e) => {
        if (e.touches.length === 1 && lbIsDragging) {
          const dx = e.touches[0].clientX - touchStartX;
          const dy = e.touches[0].clientY - touchStartY;
          if (Math.abs(dx) > 4 || Math.abs(dy) > 4) lbHasMoved = true;
          lbTranslateX = lbDragStartTx + dx;
          lbTranslateY = lbDragStartTy + dy;
          clampLightboxTranslation(lbScale);
          updateLightboxTransform(false);
        } else if (e.touches.length === 2 && initialPinchDist > 0) {
          const dist = Math.hypot(
            e.touches[0].clientX - e.touches[1].clientX,
            e.touches[0].clientY - e.touches[1].clientY
          );
          const factor = dist / initialPinchDist;
          setLightboxScale(initialPinchScale * factor);
        }
      }, { passive: true });

      lightboxViewport.addEventListener('touchend', (e) => {
        if (e.touches.length === 0) {
          if (lbScale <= 1 && !lbHasMoved) {
            const endX = e.changedTouches[0].clientX;
            const diffX = endX - touchStartX;
            if (Math.abs(diffX) > 45) {
              if (diffX > 0) {
                showLightboxImage(lbCurrentIdx - 1);
              } else {
                showLightboxImage(lbCurrentIdx + 1);
              }
            }
          }
          lbIsDragging = false;
          initialPinchDist = 0;
        }
      }, { passive: true });
    }

    // Global Keyboard Navigation
    document.addEventListener('keydown', (e) => {
      if (!lightboxModal.classList.contains('open')) return;
      if (e.key === 'Escape') {
        closeLightbox();
      } else if (e.key === 'ArrowLeft') {
        if (lbScale <= 1.1) showLightboxImage(lbCurrentIdx - 1);
      } else if (e.key === 'ArrowRight') {
        if (lbScale <= 1.1) showLightboxImage(lbCurrentIdx + 1);
      } else if (e.key === '+' || e.key === '=') {
        setLightboxScale(lbScale + 0.5);
      } else if (e.key === '-' || e.key === '_') {
        setLightboxScale(lbScale - 0.5);
      } else if (e.key === '0') {
        resetLightboxZoom(true);
      }
    });
  }

  // Trigger floating lightbox on Vehicle Detail page
  if (mainImg) {
    mainImg.addEventListener('click', (e) => {
      e.preventDefault();
      const images = getDetailCarImages();
      const carName = galleryWrapper ? (galleryWrapper.getAttribute('data-car-name') || 'Escocar') : 'Escocar';
      openLightbox(images, currentGalleryIdx, carName);
    });
  }

  if (galleryExpandBtn) {
    galleryExpandBtn.addEventListener('click', (e) => {
      e.preventDefault();
      e.stopPropagation();
      const images = getDetailCarImages();
      const carName = galleryWrapper ? (galleryWrapper.getAttribute('data-car-name') || 'Escocar') : 'Escocar';
      openLightbox(images, currentGalleryIdx, carName);
    });
  }

  window.EscocarLightbox = {
    open: openLightbox,
    close: closeLightbox
  };

  // 1b. Card Carousel ("Calecita" de fotos en catálogo y portada)
  function updateCardCarousel(carousel, newIndex, images) {
    if (!carousel || !images || images.length === 0) return;
    if (newIndex < 0) newIndex = images.length - 1;
    if (newIndex >= images.length) newIndex = 0;

    carousel.setAttribute('data-current-index', newIndex);

    const imgEl = carousel.querySelector('.card-carousel-img');
    if (imgEl && images[newIndex]) {
      imgEl.style.opacity = '0.5';
      const rawUrl = images[newIndex];
      imgEl.src = (rawUrl.startsWith('/uploads/') && !rawUrl.includes('?')) ? `${rawUrl}?thumb=1` : rawUrl;
      imgEl.onload = () => { imgEl.style.opacity = '1'; };
      setTimeout(() => { imgEl.style.opacity = '1'; }, 200);
    }

    const currentSpan = carousel.querySelector('.carousel-current');
    if (currentSpan) {
      currentSpan.textContent = newIndex + 1;
    }

    const dots = carousel.querySelectorAll('.carousel-dot');
    dots.forEach((dot, idx) => {
      dot.classList.toggle('active', idx === newIndex);
    });
  }

  document.addEventListener('click', (e) => {
    // Expand / Lightbox button in card carousel
    const expandBtn = e.target.closest('.card-carousel-expand');
    if (expandBtn) {
      e.preventDefault();
      e.stopPropagation();
      const carousel = expandBtn.closest('.card-carousel');
      if (!carousel) return;

      let images = [];
      try {
        images = JSON.parse(carousel.getAttribute('data-images') || '[]');
      } catch (err) {
        images = [];
      }
      if (images.length === 0) {
        const img = carousel.querySelector('img');
        if (img && img.src) images = [img.src];
      }
      const currentIdx = parseInt(carousel.getAttribute('data-current-index') || '0', 10);
      const carName = carousel.getAttribute('data-car-name') || 'Escocar';
      openLightbox(images, currentIdx, carName);
      return;
    }

    // Prev / Next button in card carousel
    const navBtn = e.target.closest('.card-carousel-nav');
    if (navBtn) {
      e.preventDefault();
      e.stopPropagation();
      const carousel = navBtn.closest('.card-carousel');
      if (!carousel) return;

      let images = [];
      try {
        images = JSON.parse(carousel.getAttribute('data-images') || '[]');
      } catch (err) {
        images = [];
      }
      if (images.length <= 1) return;

      let currentIdx = parseInt(carousel.getAttribute('data-current-index') || '0', 10);
      if (navBtn.classList.contains('prev')) {
        updateCardCarousel(carousel, currentIdx - 1, images);
      } else {
        updateCardCarousel(carousel, currentIdx + 1, images);
      }
      return;
    }

    // Dot click in card carousel
    const dot = e.target.closest('.carousel-dot');
    if (dot) {
      e.preventDefault();
      e.stopPropagation();
      const carousel = dot.closest('.card-carousel');
      if (!carousel) return;

      let images = [];
      try {
        images = JSON.parse(carousel.getAttribute('data-images') || '[]');
      } catch (err) {
        images = [];
      }
      const targetIdx = parseInt(dot.getAttribute('data-index') || '0', 10);
      updateCardCarousel(carousel, targetIdx, images);
    }
  });

  // Mobile Touch Swipe support on card carousel
  document.querySelectorAll('.car-thumb.card-carousel').forEach(carousel => {
    let startX = 0;
    carousel.addEventListener('touchstart', (e) => {
      startX = e.changedTouches[0].screenX;
    }, { passive: true });

    carousel.addEventListener('touchend', (e) => {
      const endX = e.changedTouches[0].screenX;
      const diffX = endX - startX;
      if (Math.abs(diffX) > 40) {
        let images = [];
        try {
          images = JSON.parse(carousel.getAttribute('data-images') || '[]');
        } catch (err) {
          images = [];
        }
        if (images.length <= 1) return;
        let currentIdx = parseInt(carousel.getAttribute('data-current-index') || '0', 10);
        if (diffX > 0) {
          updateCardCarousel(carousel, currentIdx - 1, images);
        } else {
          updateCardCarousel(carousel, currentIdx + 1, images);
        }
      }
    }, { passive: true });
  });


  // 3. Lead Contact Modal & Mobile Bottom Sheet
  const modalBackdrop = document.getElementById('leadModal');
  const openModalBtns = document.querySelectorAll('[data-open-modal="lead"]');
  const closeModalBtn = document.getElementById('closeLeadModal');
  const leadForm = document.getElementById('leadForm');
  const modalCarTitle = document.getElementById('modalCarTitle');
  const modalCarIdInput = document.getElementById('modalCarIdInput');
  const modalQuickWaBtn = document.getElementById('modalQuickWaBtn');

  function openLeadModal(carId, carName) {
    if (!modalBackdrop) return;
    checkSavedLeadProfile();
    if (modalCarTitle && carName) modalCarTitle.textContent = carName;
    if (modalCarIdInput && carId) modalCarIdInput.value = carId;

    if (modalQuickWaBtn) {
      const baseWa = modalQuickWaBtn.getAttribute('data-base-wa') || '5491100000000';
      if (carName) {
        modalQuickWaBtn.href = `https://wa.me/${baseWa}?text=${encodeURIComponent('Hola Escocar, quisiera consultar por el vehículo ' + carName)}`;
      } else {
        modalQuickWaBtn.href = `https://wa.me/${baseWa}?text=${encodeURIComponent('Hola Escocar, quisiera recibir asesoramiento comercial')}`;
      }
    }

    document.body.classList.add('modal-open');
    modalBackdrop.classList.add('open');
  }

  function closeLeadModal() {
    if (!modalBackdrop) return;
    modalBackdrop.classList.remove('open');
    document.body.classList.remove('modal-open');
  }

  // Auto-fill from localStorage if available
  function checkSavedLeadProfile() {
    try {
      const saved = JSON.parse(localStorage.getItem('escocar_lead_profile') || 'null');
      if (saved) {
        const nameInput = document.getElementById('leadName');
        const phoneInput = document.getElementById('leadPhone');
        const emailInput = document.getElementById('leadEmail');
        const badge = document.getElementById('returningLeadBadge');

        if (nameInput && !nameInput.value && saved.name) nameInput.value = saved.name;
        if (phoneInput && !phoneInput.value && saved.phone) phoneInput.value = saved.phone;
        if (emailInput && !emailInput.value && saved.email) emailInput.value = saved.email;
        if (badge && (saved.name || saved.phone)) badge.style.display = 'flex';
      }
    } catch (e) {}
  }

  if (modalBackdrop) {
    checkSavedLeadProfile();

    openModalBtns.forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.preventDefault();
        const carId = btn.getAttribute('data-car-id');
        const carName = btn.getAttribute('data-car-name');
        openLeadModal(carId, carName);
      });
    });

    if (closeModalBtn) {
      closeModalBtn.addEventListener('click', closeLeadModal);
    }

    modalBackdrop.addEventListener('click', (e) => {
      if (e.target === modalBackdrop) {
        closeLeadModal();
      }
    });

    // Close on Escape key
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape' && modalBackdrop.classList.contains('open')) {
        closeLeadModal();
      }
    });

    if (leadForm) {
      leadForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const submitBtn = leadForm.querySelector('button[type="submit"]');
        const originalText = submitBtn.textContent;
        submitBtn.disabled = true;
        submitBtn.textContent = 'Enviando...';

        const carIdVal = modalCarIdInput ? parseInt(modalCarIdInput.value, 10) : null;
        const payload = {
          car_id: isNaN(carIdVal) ? null : carIdVal,
          name: document.getElementById('leadName').value,
          phone: document.getElementById('leadPhone').value,
          email: document.getElementById('leadEmail').value,
          message: document.getElementById('leadMessage').value
        };

        try {
          const res = await fetch('/api/leads', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
          });
          if (res.ok) {
            // Save lead profile locally for instant auto-complete in future journeys
            try {
              localStorage.setItem('escocar_lead_profile', JSON.stringify({
                name: payload.name,
                phone: payload.phone,
                email: payload.email
              }));
            } catch (e) {}

            if (window.EscocarTracker) {
              window.EscocarTracker.sendEvent('lead_form_submitted', { name: payload.name, phone: payload.phone }, payload.car_id);
            }

            leadForm.innerHTML = `
              <div style="text-align: center; padding: 2rem 0;">
                <div style="font-size: 3rem; color: #10b981; margin-bottom: 1rem;">✓</div>
                <h3>¡Mensaje Enviado con Éxito!</h3>
                <p style="margin-top: 0.5rem; color: #94a3b8;">Un asesor comercial de Escocar se contactará contigo a la brevedad por WhatsApp o llamada.</p>
                <button type="button" class="btn btn-secondary" style="margin-top: 1.5rem;" id="successModalCloseBtn">Cerrar</button>
              </div>
            `;
            const sBtn = document.getElementById('successModalCloseBtn');
            if (sBtn) sBtn.addEventListener('click', closeLeadModal);
          } else {
            alert('Hubo un error al enviar tu consulta. Por favor intenta por WhatsApp.');
            submitBtn.disabled = false;
            submitBtn.textContent = originalText;
          }
        } catch (err) {
          alert('Error de conexión. Por favor contáctanos por WhatsApp.');
          submitBtn.disabled = false;
          submitBtn.textContent = originalText;
        }
      });
    }
  }

  // 4. Mobile Navigation Drawer Toggle
  const navToggle = document.getElementById('mobileNavToggle');
  const navDrawer = document.getElementById('mobileNavDrawer');
  const navBackdrop = document.getElementById('mobileNavBackdrop');
  const navClose = document.getElementById('mobileNavClose');

  function toggleMobileNav(open) {
    if (!navDrawer) return;
    navDrawer.classList.toggle('open', open);
    if (navBackdrop) navBackdrop.classList.toggle('open', open);
    if (navToggle) navToggle.classList.toggle('open', open);
    document.body.classList.toggle('nav-drawer-open', open);
  }

  if (navToggle) {
    navToggle.addEventListener('click', () => {
      const isOpen = navDrawer && navDrawer.classList.contains('open');
      toggleMobileNav(!isOpen);
    });
  }

  if (navClose) {
    navClose.addEventListener('click', () => toggleMobileNav(false));
  }

  if (navBackdrop) {
    navBackdrop.addEventListener('click', () => toggleMobileNav(false));
  }

  document.querySelectorAll('.mobile-nav-link').forEach(link => {
    link.addEventListener('click', () => toggleMobileNav(false));
  });

  // 5. First Visit of the Day - Sell Your Car Prompt
  const dailySellPrompt = document.getElementById('dailySellPrompt');
  const closeDailySellPromptBtn = document.getElementById('closeDailySellPrompt');
  const dailySellLeadLink = document.getElementById('dailySellLeadLink');
  const dailySellWaLink = document.getElementById('dailySellWaLink');

  function hideDailySellPrompt() {
    if (!dailySellPrompt) return;
    dailySellPrompt.classList.remove('visible');
    dailySellPrompt.setAttribute('aria-hidden', 'true');
  }

  if (dailySellPrompt) {
    const now = new Date();
    const todayKey = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`;
    let lastShownDate = null;
    try {
      lastShownDate = localStorage.getItem('escocar_daily_sell_prompt_date');
    } catch (e) {}

    if (lastShownDate !== todayKey) {
      setTimeout(() => {
        // Don't interrupt if user already opened the lead modal or lightbox
        if (modalBackdrop && modalBackdrop.classList.contains('open')) return;
        dailySellPrompt.classList.add('visible');
        dailySellPrompt.setAttribute('aria-hidden', 'false');
        try {
          localStorage.setItem('escocar_daily_sell_prompt_date', todayKey);
        } catch (e) {}
      }, 3500);
    }

    if (closeDailySellPromptBtn) {
      closeDailySellPromptBtn.addEventListener('click', hideDailySellPrompt);
    }

    if (dailySellLeadLink) {
      dailySellLeadLink.addEventListener('click', (e) => {
        e.preventDefault();
        hideDailySellPrompt();
        openLeadModal(null, 'Vender mi Auto / Tasación');
      });
    }

    if (dailySellWaLink) {
      dailySellWaLink.addEventListener('click', () => {
        hideDailySellPrompt();
      });
    }
  }

  // Quick Action for Admin Draft Preview: Publish directly from the page
  window.publishDraftCar = async function(carId) {
    if (!carId) return;
    if (!confirm('¿Deseás publicar este vehículo ahora en el catálogo web?')) return;
    try {
      const res = await fetch(`/api/cars/${carId}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status: 'disponible' })
      });
      if (res.ok) {
        alert('¡Vehículo publicado exitosamente en el catálogo!');
        window.location.reload();
      } else {
        alert('Error al publicar: ' + (await res.text()));
      }
    } catch (err) {
      alert('Error de conexión al publicar');
    }
  };
});
