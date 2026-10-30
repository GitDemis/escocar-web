/**
 * Escocar Analytics & Visitor Tracking Agent
 * Handles client telemetry, anonymous visitor persistence, and interest beacons.
 */
(function() {
  'use strict';

  function getCookie(name) {
    const match = document.cookie.match(new RegExp('(^| )' + name + '=([^;]+)'));
    return match ? decodeURIComponent(match[2]) : null;
  }

  const tracker = {
    vid: getCookie('escocar_vid'),
    sid: getCookie('escocar_sid'),
    startTime: Date.now(),

    sendEvent: function(eventType, eventData, carId) {
      const payload = {
        event_type: eventType,
        event_data: typeof eventData === 'object' ? JSON.stringify(eventData) : (eventData || ''),
        car_id: carId ? parseInt(carId, 10) : null
      };

      if (navigator.sendBeacon) {
        const blob = new Blob([JSON.stringify(payload)], { type: 'application/json' });
        navigator.sendBeacon('/api/track/event', blob);
      } else {
        fetch('/api/track/event', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        }).catch(() => {});
      }

      // Relay to Google Analytics 4 if gtag is loaded
      if (typeof window.gtag === 'function') {
        try {
          if (eventType === 'whatsapp_click') {
            window.gtag('event', 'generate_lead', {
              event_category: 'engagement',
              event_label: (typeof eventData === 'object' && eventData.car) ? eventData.car : 'WhatsApp Inquiry',
              method: 'whatsapp',
              car_id: carId || null
            });
          } else if (eventType === 'lead_form_submitted') {
            window.gtag('event', 'generate_lead', {
              event_category: 'conversion',
              event_label: 'Web Form Lead',
              method: 'form',
              car_id: carId || null
            });
          } else if (eventType === 'view_detail_click') {
            window.gtag('event', 'select_content', {
              content_type: 'car',
              item_id: carId ? String(carId) : ''
            });
          } else if (eventType === 'finance_calculate' || eventType === 'finance_inquiry') {
            window.gtag('event', 'calculate_financing', {
              event_category: 'engagement',
              event_label: 'Consulta de Financiación',
              car_id: carId || null
            });
          } else {
            window.gtag('event', eventType, {
              event_category: 'custom',
              car_id: carId || null
            });
          }
        } catch (err) {
          // Silent catch for analytics errors
        }
      }
    },

    init: function() {
      // 1. Track WhatsApp clicks
      document.addEventListener('click', (e) => {
        const waBtn = e.target.closest('[data-track="whatsapp"]');
        if (waBtn) {
          const carId = waBtn.getAttribute('data-car-id');
          const carName = waBtn.getAttribute('data-car-name') || 'General';
          tracker.sendEvent('whatsapp_click', { car: carName, target: waBtn.href }, carId);
        }

        // Track car card view clicks
        const detailBtn = e.target.closest('[data-track="car_view"]');
        if (detailBtn) {
          const carId = detailBtn.getAttribute('data-car-id');
          tracker.sendEvent('view_detail_click', {}, carId);
        }
      });

      // 2. Track car detail view in GA4
      const carElem = document.querySelector('[data-current-car-id]');
      if (carElem && typeof window.gtag === 'function') {
        const carId = carElem.getAttribute('data-current-car-id');
        window.gtag('event', 'view_item', {
          item_id: carId,
          item_name: document.title
        });
      }

      // 3. Track page duration on leave
      const sendDuration = () => {
        const durationSec = Math.round((Date.now() - tracker.startTime) / 1000);
        if (durationSec > 2) {
          const currentCar = document.querySelector('[data-current-car-id]');
          const carId = currentCar ? currentCar.getAttribute('data-current-car-id') : null;
          tracker.sendEvent('page_duration', { seconds: durationSec, path: window.location.pathname }, carId);
        }
      };

      window.addEventListener('visibilitychange', () => {
        if (document.visibilityState === 'hidden') {
          sendDuration();
        }
      });
      window.addEventListener('beforeunload', sendDuration);
    }
  };

  // Expose global tracker instance
  window.EscocarTracker = tracker;
  document.addEventListener('DOMContentLoaded', tracker.init);
})();
