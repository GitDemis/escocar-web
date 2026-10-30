/**
 * Escocar Autos - Main Application Scripts
 */
document.addEventListener('DOMContentLoaded', () => {
  'use strict';

  // 1. Gallery Switcher & Navigation on Vehicle Detail Page
  const mainImg = document.getElementById('mainGalleryImg');
  const thumbs = document.querySelectorAll('.thumb-item');
  const galleryPrevBtn = document.getElementById('galleryPrevBtn');
  const galleryNextBtn = document.getElementById('galleryNextBtn');
  const galleryCounterNum = document.getElementById('galleryCurrentNum');
  const galleryWrapper = document.getElementById('mainGalleryWrapper');

  if (mainImg && thumbs.length > 0) {
    let currentGalleryIdx = 0;

    function setGalleryImage(idx) {
      if (idx < 0) idx = thumbs.length - 1;
      if (idx >= thumbs.length) idx = 0;
      currentGalleryIdx = idx;

      thumbs.forEach((t, i) => {
        t.classList.toggle('active', i === idx);
      });

      const selectedThumb = thumbs[idx];
      if (selectedThumb) {
        const newSrc = selectedThumb.getAttribute('data-img');
        if (newSrc) {
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

    thumbs.forEach((thumb, i) => {
      thumb.addEventListener('click', () => {
        setGalleryImage(i);
      });
    });

    if (galleryPrevBtn) {
      galleryPrevBtn.addEventListener('click', (e) => {
        e.preventDefault();
        setGalleryImage(currentGalleryIdx - 1);
      });
    }

    if (galleryNextBtn) {
      galleryNextBtn.addEventListener('click', (e) => {
        e.preventDefault();
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
            setGalleryImage(currentGalleryIdx - 1); // swipe right -> previous
          } else {
            setGalleryImage(currentGalleryIdx + 1); // swipe left -> next
          }
        }
      }, { passive: true });
    }
  }

  // 1b. Card Carousel ("Calecita" de fotos en catálogo y portada)
  function updateCardCarousel(carousel, newIndex, images) {
    if (!carousel || !images || images.length === 0) return;
    if (newIndex < 0) newIndex = images.length - 1;
    if (newIndex >= images.length) newIndex = 0;

    carousel.setAttribute('data-current-index', newIndex);

    const imgEl = carousel.querySelector('.card-carousel-img');
    if (imgEl && images[newIndex]) {
      imgEl.style.opacity = '0.5';
      imgEl.src = images[newIndex];
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
});
