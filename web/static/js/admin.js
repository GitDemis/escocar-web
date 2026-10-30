/**
 * Escocar Autos - Admin Dashboard Scripts
 */
document.addEventListener('DOMContentLoaded', () => {
  'use strict';

  // 1. Tab Navigation
  const tabs = document.querySelectorAll('.admin-tab');
  const panels = document.querySelectorAll('.tab-content');

  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      const target = tab.getAttribute('data-tab');
      tabs.forEach(t => t.classList.remove('active'));
      panels.forEach(p => p.classList.remove('active'));

      tab.classList.add('active');
      const panel = document.getElementById(target);
      if (panel) panel.classList.add('active');
    });
  });

  // Helpers for file uploads
  function isImageFile(file) {
    return file && file.type && (file.type.startsWith('image/jpeg') || file.type.startsWith('image/png') || file.type.startsWith('image/webp'));
  }

  // 2. Add New Car Uploader State & Form
  let addCarFiles = [];
  const addCarDropzone = document.getElementById('addCarDropzone');
  const addCarFileInput = document.getElementById('addCarFileInput');
  const addCarPreviewGrid = document.getElementById('addCarPreviewGrid');

  function renderAddCarPreviews() {
    if (!addCarPreviewGrid) return;
    addCarPreviewGrid.innerHTML = '';
    addCarFiles.forEach((file, idx) => {
      const card = document.createElement('div');
      card.className = 'image-preview-card';
      const objUrl = URL.createObjectURL(file);
      card.innerHTML = `
        <img src="${objUrl}" alt="${file.name}">
        <button type="button" class="btn-remove-img" data-remove-add-idx="${idx}" title="Quitar foto">✕</button>
        <span class="img-badge">${idx === 0 ? 'Portada' : idx + 1}</span>
      `;
      addCarPreviewGrid.appendChild(card);
    });
  }

  if (addCarDropzone && addCarFileInput) {
    addCarFileInput.addEventListener('change', (e) => {
      const selected = Array.from(e.target.files).filter(isImageFile);
      addCarFiles = addCarFiles.concat(selected);
      renderAddCarPreviews();
      addCarFileInput.value = '';
    });

    addCarDropzone.addEventListener('dragover', (e) => {
      e.preventDefault();
      addCarDropzone.classList.add('drag-over');
    });

    addCarDropzone.addEventListener('dragleave', () => {
      addCarDropzone.classList.remove('drag-over');
    });

    addCarDropzone.addEventListener('drop', (e) => {
      e.preventDefault();
      addCarDropzone.classList.remove('drag-over');
      if (e.dataTransfer && e.dataTransfer.files) {
        const dropped = Array.from(e.dataTransfer.files).filter(isImageFile);
        addCarFiles = addCarFiles.concat(dropped);
        renderAddCarPreviews();
      }
    });

    if (addCarPreviewGrid) {
      addCarPreviewGrid.addEventListener('click', (e) => {
        const btn = e.target.closest('[data-remove-add-idx]');
        if (btn) {
          const idx = parseInt(btn.getAttribute('data-remove-add-idx'), 10);
          addCarFiles.splice(idx, 1);
          renderAddCarPreviews();
        }
      });
    }
  }

  const addCarForm = document.getElementById('addCarForm');
  if (addCarForm) {
    addCarForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const submitBtn = addCarForm.querySelector('button[type="submit"]');
      const origText = submitBtn.textContent;
      submitBtn.disabled = true;
      submitBtn.textContent = 'Guardando vehículo...';

      const formData = new FormData(addCarForm);
      formData.delete('images');
      addCarFiles.forEach(file => {
        formData.append('images', file, file.name);
      });

      try {
        const res = await fetch('/api/cars', {
          method: 'POST',
          body: formData
        });

        if (res.ok) {
          alert('¡Vehículo agregado exitosamente al inventario!');
          window.location.reload();
        } else {
          const errText = await res.text();
          alert('Error al guardar vehículo: ' + errText);
          submitBtn.disabled = false;
          submitBtn.textContent = origText;
        }
      } catch (err) {
        alert('Error de red al conectar con el servidor');
        submitBtn.disabled = false;
        submitBtn.textContent = origText;
      }
    });
  }

  // 3. Archive Car Action
  function syncCarRowActions(row, status) {
    if (!row) return;
    const archiveBtn = row.querySelector('[data-delete-car]');
    const purgeBtn = row.querySelector('[data-purge-car]');
    const isArchived = status === 'archivado';
    if (archiveBtn) archiveBtn.style.display = isArchived ? 'none' : '';
    if (purgeBtn) purgeBtn.style.display = isArchived ? '' : 'none';
  }

  document.addEventListener('click', async (e) => {
    const deleteBtn = e.target.closest('[data-delete-car]');
    if (deleteBtn) {
      const carId = deleteBtn.getAttribute('data-delete-car');
      if (confirm(`¿Estás seguro de archivar la publicación #${carId}?\n\nLa publicación dejará de ser visible para los clientes y quedará guardada como archivada solo para los administradores.`)) {
        try {
          const res = await fetch(`/api/cars/${carId}`, {
            method: 'DELETE'
          });
          if (res.ok) {
            const row = deleteBtn.closest('tr');
            if (row) {
              const statusSelect = row.querySelector('.admin-status-select');
              if (statusSelect) {
                statusSelect.value = 'archivado';
                statusSelect.setAttribute('data-status', 'archivado');
                statusSelect.style.boxShadow = '0 0 12px rgba(148, 163, 184, 0.8)';
                setTimeout(() => { statusSelect.style.boxShadow = 'none'; }, 1500);
              }
              syncCarRowActions(row, 'archivado');
            }
          } else {
            alert('Error al archivar la publicación');
          }
        } catch (err) {
          alert('Error de conexión');
        }
      }
    }
  });

  // 3a. Permanently Delete Archived Car
  document.addEventListener('click', async (e) => {
    const purgeBtn = e.target.closest('[data-purge-car]');
    if (!purgeBtn) return;
    const carId = purgeBtn.getAttribute('data-purge-car');
    const row = purgeBtn.closest('tr');
    const title = row ? (row.querySelector('strong')?.textContent || '').trim() : '';
    if (!confirm(`⚠️ ¿Eliminar DEFINITIVAMENTE la publicación #${carId}${title ? ' (' + title + ')' : ''}?\n\nSe borrarán el vehículo y sus fotos. Esta acción NO se puede deshacer.`)) return;

    purgeBtn.disabled = true;
    try {
      const res = await fetch(`/api/cars/${carId}?permanent=1`, { method: 'DELETE' });
      if (res.ok) {
        if (row) {
          row.classList.add('row-removing');
          setTimeout(() => row.remove(), 350);
        }
      } else {
        alert('No se pudo eliminar: ' + (await res.text()));
        purgeBtn.disabled = false;
      }
    } catch (err) {
      alert('Error de conexión al eliminar');
      purgeBtn.disabled = false;
    }
  });

  // 3b. Edit Car Modal & Uploader State
  const editCarModal = document.getElementById('editCarModal');
  const editCarForm = document.getElementById('editCarForm');
  const closeEditCarModalBtn = document.getElementById('closeEditCarModal');
  const cancelEditCarBtn = document.getElementById('cancelEditCarBtn');
  const editCarExistingGrid = document.getElementById('editCarExistingGrid');
  const editCarExistingCount = document.getElementById('editCarExistingCount');
  const editCarExistingImagesInput = document.getElementById('editCarExistingImagesInput');
  const editCarDropzone = document.getElementById('editCarDropzone');
  const editCarFileInput = document.getElementById('editCarFileInput');
  const editCarPreviewGrid = document.getElementById('editCarPreviewGrid');

  let editCarExistingList = [];
  let editCarNewFiles = [];

  function renderEditCarExisting() {
    if (!editCarExistingGrid) return;
    editCarExistingGrid.innerHTML = '';
    if (editCarExistingCount) {
      editCarExistingCount.textContent = `${editCarExistingList.length} fotos conservadas`;
    }
    if (editCarExistingImagesInput) {
      editCarExistingImagesInput.value = JSON.stringify(editCarExistingList);
    }
    if (editCarExistingList.length === 0) {
      editCarExistingGrid.innerHTML = '<div style="color: var(--text-dim); font-size: 0.85rem; padding: 0.5rem; grid-column: 1 / -1;">No hay fotos previas conservadas.</div>';
      return;
    }
    editCarExistingList.forEach((imgUrl, idx) => {
      const card = document.createElement('div');
      card.className = 'image-preview-card';
      card.innerHTML = `
        <img src="${imgUrl}" alt="Foto ${idx}">
        <button type="button" class="btn-remove-img" data-remove-existing-idx="${idx}" title="Eliminar esta foto">🗑️</button>
        <span class="img-badge">${idx === 0 ? 'Portada' : idx + 1}</span>
      `;
      editCarExistingGrid.appendChild(card);
    });
  }

  function renderEditCarNewPreviews() {
    if (!editCarPreviewGrid) return;
    editCarPreviewGrid.innerHTML = '';
    editCarNewFiles.forEach((file, idx) => {
      const card = document.createElement('div');
      card.className = 'image-preview-card';
      const objUrl = URL.createObjectURL(file);
      card.innerHTML = `
        <img src="${objUrl}" alt="${file.name}">
        <button type="button" class="btn-remove-img" data-remove-new-idx="${idx}" title="Quitar foto">✕</button>
        <span class="img-badge" style="background: rgba(16, 185, 129, 0.85); color: #fff;">Nueva</span>
      `;
      editCarPreviewGrid.appendChild(card);
    });
  }

  if (editCarExistingGrid) {
    editCarExistingGrid.addEventListener('click', (e) => {
      const btn = e.target.closest('[data-remove-existing-idx]');
      if (btn) {
        const idx = parseInt(btn.getAttribute('data-remove-existing-idx'), 10);
        editCarExistingList.splice(idx, 1);
        renderEditCarExisting();
      }
    });
  }

  if (editCarDropzone && editCarFileInput) {
    editCarFileInput.addEventListener('change', (e) => {
      const selected = Array.from(e.target.files).filter(isImageFile);
      editCarNewFiles = editCarNewFiles.concat(selected);
      renderEditCarNewPreviews();
      editCarFileInput.value = '';
    });

    editCarDropzone.addEventListener('dragover', (e) => {
      e.preventDefault();
      editCarDropzone.classList.add('drag-over');
    });

    editCarDropzone.addEventListener('dragleave', () => {
      editCarDropzone.classList.remove('drag-over');
    });

    editCarDropzone.addEventListener('drop', (e) => {
      e.preventDefault();
      editCarDropzone.classList.remove('drag-over');
      if (e.dataTransfer && e.dataTransfer.files) {
        const dropped = Array.from(e.dataTransfer.files).filter(isImageFile);
        editCarNewFiles = editCarNewFiles.concat(dropped);
        renderEditCarNewPreviews();
      }
    });

    if (editCarPreviewGrid) {
      editCarPreviewGrid.addEventListener('click', (e) => {
        const btn = e.target.closest('[data-remove-new-idx]');
        if (btn) {
          const idx = parseInt(btn.getAttribute('data-remove-new-idx'), 10);
          editCarNewFiles.splice(idx, 1);
          renderEditCarNewPreviews();
        }
      });
    }
  }

  function safeSetVal(id, val) {
    const el = document.getElementById(id);
    if (el) el.value = (val !== undefined && val !== null) ? val : '';
  }

  window.closeEditCarModal = function() {
    const modal = document.getElementById('editCarModal');
    if (modal) {
      modal.classList.remove('active');
    }
  };

  window.openEditCarModal = async function(carId) {
    if (!carId) return;
    const editBtn = document.querySelector(`[data-edit-car="${carId}"]`);
    try {
      if (editBtn) editBtn.disabled = true;
      const res = await fetch(`/api/cars/${carId}`);
      if (!res.ok) {
        throw new Error('No se pudo cargar la información del vehículo (HTTP ' + res.status + ')');
      }
      const car = await res.json();
      
      safeSetVal('editCarId', car.id);
      const badge = document.getElementById('editCarHeaderBadge');
      if (badge) badge.textContent = `ID #${car.id}`;
      safeSetVal('editCarMake', car.make);
      safeSetVal('editCarModel', car.model);
      safeSetVal('editCarCurrency', (car.currency && car.currency.toUpperCase() === 'ARS') ? 'ARS' : 'USD');
      safeSetVal('editCarPrice', car.price);
      safeSetVal('editCarDownPayment', car.down_payment || 0);
      safeSetVal('editCarYear', car.year);
      safeSetVal('editCarMileage', car.mileage || 0);
      safeSetVal('editCarDoors', car.doors || 4);
      safeSetVal('editCarBody', car.body_type || 'SUV');
      safeSetVal('editCarFuel', car.fuel_type || 'Nafta');
      safeSetVal('editCarTrans', car.transmission || 'Automática');
      safeSetVal('editCarColor', car.color);
      safeSetVal('editCarEngine', car.engine);
      safeSetVal('editCarStatus', car.status || 'disponible');
      safeSetVal('editCarFeatured', car.featured ? 'true' : 'false');
      safeSetVal('editCarDescription', car.description);
      safeSetVal('editCarFeatures', (car.features || []).join('\n'));

      // Populate existing photos and clear new files
      editCarExistingList = Array.isArray(car.images) ? [...car.images] : [];
      editCarNewFiles = [];
      renderEditCarExisting();
      renderEditCarNewPreviews();

      const modal = document.getElementById('editCarModal');
      if (modal) modal.classList.add('active');
    } catch (err) {
      console.error('Error al abrir editor de auto:', err);
      alert('Error al abrir editor: ' + err.message);
    } finally {
      if (editBtn) editBtn.disabled = false;
    }
  };

  if (closeEditCarModalBtn) closeEditCarModalBtn.addEventListener('click', window.closeEditCarModal);
  if (cancelEditCarBtn) cancelEditCarBtn.addEventListener('click', window.closeEditCarModal);
  if (editCarModal) {
    editCarModal.addEventListener('click', (e) => {
      if (e.target === editCarModal) window.closeEditCarModal();
    });
  }

  document.addEventListener('click', (e) => {
    const editBtn = e.target.closest('[data-edit-car]');
    if (editBtn) {
      e.preventDefault();
      const carId = editBtn.getAttribute('data-edit-car');
      if (window.openEditCarModal) {
        window.openEditCarModal(carId);
      }
    }
  });

  if (editCarForm) {
    editCarForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const carId = document.getElementById('editCarId').value;
      const saveBtn = document.getElementById('saveEditCarBtn');
      const origText = saveBtn.textContent;
      saveBtn.disabled = true;
      saveBtn.textContent = 'Guardando cambios...';

      const formData = new FormData(editCarForm);
      formData.delete('images');
      formData.set('existing_images', JSON.stringify(editCarExistingList));
      editCarNewFiles.forEach(file => {
        formData.append('images', file, file.name);
      });

      try {
        const res = await fetch(`/api/cars/${carId}`, {
          method: 'POST',
          body: formData
        });

        if (res.ok) {
          alert('¡Vehículo actualizado con éxito!');
          window.location.reload();
        } else {
          const errText = await res.text();
          alert('Error al guardar cambios: ' + errText);
          saveBtn.disabled = false;
          saveBtn.textContent = origText;
        }
      } catch (err) {
        alert('Error de conexión al guardar vehículo: ' + err.message);
        saveBtn.disabled = false;
        saveBtn.textContent = origText;
      }
    });
  }

  // 4. Quick Status Change Action
  document.addEventListener('change', async (e) => {
    const statusSelect = e.target.closest('.admin-status-select');
    if (statusSelect) {
      const carId = statusSelect.getAttribute('data-car-id');
      const newStatus = statusSelect.value;
      const prevStatus = statusSelect.getAttribute('data-status');
      statusSelect.disabled = true;

      try {
        const res = await fetch(`/api/cars/${carId}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ status: newStatus })
        });

        if (res.ok) {
          statusSelect.setAttribute('data-status', newStatus);
          syncCarRowActions(statusSelect.closest('tr'), newStatus);
          // Quick visual success indicator
          statusSelect.style.boxShadow = '0 0 10px rgba(16, 185, 129, 0.6)';
          setTimeout(() => {
            statusSelect.style.boxShadow = 'none';
          }, 1000);
        } else {
          alert('Error al actualizar el estado del vehículo');
          statusSelect.value = prevStatus;
        }
      } catch (err) {
        alert('Error de conexión al actualizar estado');
        statusSelect.value = prevStatus;
      } finally {
        statusSelect.disabled = false;
      }
    }
  });

  // 5. General Settings Form
  const settingsForm = document.getElementById('settingsForm');
  if (settingsForm) {
    const waInput = document.getElementById('settingWhatsApp');
    const gaInput = document.getElementById('settingGoogleAnalytics');
    const lookerInput = document.getElementById('settingLookerStudio');
    const testBtn = document.getElementById('testWhatsAppBtn');
    const statusSpan = document.getElementById('settingsSaveStatus');
    const saveBtn = document.getElementById('saveSettingsBtn');

    if (waInput && testBtn) {
      waInput.addEventListener('input', () => {
        const clean = waInput.value.replace(/[^0-9]/g, '');
        testBtn.href = `https://wa.me/${clean}`;
      });
    }

    settingsForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      if (!saveBtn || !waInput) return;

      saveBtn.disabled = true;
      const originalText = saveBtn.textContent;
      saveBtn.textContent = 'Guardando...';
      if (statusSpan) statusSpan.style.display = 'none';

      const payload = {
        whatsapp_number: waInput.value,
        google_analytics_id: gaInput ? gaInput.value.trim().toUpperCase() : '',
        looker_studio_url: lookerInput ? lookerInput.value.trim() : ''
      };

      try {
        const res = await fetch('/api/settings', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });

        if (res.ok) {
          const data = await res.json();
          waInput.value = data.whatsapp_number;
          if (gaInput) gaInput.value = data.google_analytics_id || '';
          if (lookerInput) lookerInput.value = data.looker_studio_url || '';
          const quickGa = document.getElementById('gaMeasurementIdInput');
          if (quickGa) quickGa.value = data.google_analytics_id || '';
          const quickLooker = document.getElementById('lookerStudioUrlInput');
          if (quickLooker) quickLooker.value = data.looker_studio_url || '';

          if (testBtn) testBtn.href = `https://wa.me/${data.whatsapp_number}`;
          if (statusSpan) {
            statusSpan.style.display = 'inline';
            statusSpan.style.color = '#34d399';
            statusSpan.textContent = '✓ Configuración guardada';
            setTimeout(() => { statusSpan.style.display = 'none'; }, 3000);
          }
        } else {
          const errText = await res.text();
          alert('Error al guardar: ' + errText);
        }
      } catch (err) {
        alert('Error de conexión al guardar configuración');
      } finally {
        saveBtn.disabled = false;
        saveBtn.textContent = originalText;
      }
    });
  }

  // 5b. Google Analytics Quick Form
  const gaQuickForm = document.getElementById('gaQuickForm');
  if (gaQuickForm) {
    const gaInput = document.getElementById('gaMeasurementIdInput');
    const saveBtn = document.getElementById('saveGaQuickBtn');
    const statusSpan = document.getElementById('gaSaveStatus');

    gaQuickForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      if (!saveBtn || !gaInput) return;

      saveBtn.disabled = true;
      const originalText = saveBtn.textContent;
      saveBtn.textContent = 'Guardando...';
      if (statusSpan) statusSpan.style.display = 'none';

      try {
        const res = await fetch('/api/settings', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ google_analytics_id: gaInput.value.trim().toUpperCase() })
        });

        if (res.ok) {
          const data = await res.json();
          gaInput.value = data.google_analytics_id || '';
          const settingGa = document.getElementById('settingGoogleAnalytics');
          if (settingGa) settingGa.value = data.google_analytics_id || '';

          if (statusSpan) {
            statusSpan.style.display = 'inline';
            statusSpan.style.color = '#34d399';
            statusSpan.textContent = '✓ ID guardado correctamente. Recargando...';
            setTimeout(() => { window.location.reload(); }, 1200);
          }
        } else {
          const errText = await res.text();
          alert('Error al guardar ID de Analytics: ' + errText);
        }
      } catch (err) {
        alert('Error de conexión al guardar ID de Analytics');
      } finally {
        saveBtn.disabled = false;
        saveBtn.textContent = originalText;
      }
    });
  }

  // 5c. Looker Studio Form & Toggle
  const toggleLookerBtn = document.getElementById('toggleLookerConfigBtn');
  const lookerConfigBox = document.getElementById('lookerStudioConfigBox');
  if (toggleLookerBtn && lookerConfigBox) {
    toggleLookerBtn.addEventListener('click', () => {
      const isHidden = lookerConfigBox.style.display === 'none';
      lookerConfigBox.style.display = isHidden ? 'block' : 'none';
    });
  }

  const lookerStudioForm = document.getElementById('lookerStudioForm');
  if (lookerStudioForm) {
    const lookerInput = document.getElementById('lookerStudioUrlInput');
    const saveBtn = document.getElementById('saveLookerBtn');
    const statusSpan = document.getElementById('lookerSaveStatus');

    lookerStudioForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      if (!saveBtn || !lookerInput) return;

      saveBtn.disabled = true;
      const originalText = saveBtn.textContent;
      saveBtn.textContent = 'Guardando...';
      if (statusSpan) statusSpan.style.display = 'none';

      try {
        const res = await fetch('/api/settings', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ looker_studio_url: lookerInput.value.trim() })
        });

        if (res.ok) {
          const data = await res.json();
          lookerInput.value = data.looker_studio_url || '';
          const settingLooker = document.getElementById('settingLookerStudio');
          if (settingLooker) settingLooker.value = data.looker_studio_url || '';

          if (statusSpan) {
            statusSpan.style.display = 'inline';
            statusSpan.style.color = '#34d399';
            statusSpan.textContent = '✓ Dashboard vinculado. Actualizando vista...';
            setTimeout(() => { window.location.reload(); }, 1200);
          }
        } else {
          const errText = await res.text();
          alert('Error al guardar URL de Looker Studio: ' + errText);
        }
      } catch (err) {
        alert('Error de conexión al guardar Looker Studio');
      } finally {
        saveBtn.disabled = false;
        saveBtn.textContent = originalText;
      }
    });
  }

  // 6. Leads Pipeline CRM & Management
  const STATUS_CONFIG = {
    'nuevo': { label: 'Nuevo', class: 'badge-status-nuevo' },
    'en_gestion': { label: 'En Gestión', class: 'badge-status-en_gestion' },
    'contactado': { label: 'Contactado', class: 'badge-status-contactado' },
    'visita_agendada': { label: 'Visita Agendada', class: 'badge-status-visita_agendada' },
    'cerrado_ganado': { label: 'Vendido', class: 'badge-status-cerrado_ganado' },
    'descartado': { label: 'Descartado', class: 'badge-status-descartado' },
    'archivado': { label: 'Archivado', class: 'badge-status-archivado' }
  };

  function formatLeadDate(dateStr) {
    if (!dateStr) return '-';
    try {
      const d = new Date(dateStr);
      return d.toLocaleString('es-AR', {
        timeZone: 'America/Argentina/Buenos_Aires',
        day: '2-digit', month: '2-digit', year: 'numeric',
        hour: '2-digit', minute: '2-digit', hour12: false
      });
    } catch (e) {
      return dateStr;
    }
  }

  function escapeHtml(str) {
    if (!str) return '';
    return str.replace(/[&<>'"]/g, 
      tag => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[tag] || tag)
    );
  }

  // Lead Status Filtering
  const filterBtns = document.querySelectorAll('.lead-filter-btn');
  let currentLeadFilter = 'all';

  function applyLeadFilter() {
    document.querySelectorAll('.lead-table-row').forEach(row => {
      const rowStatus = row.getAttribute('data-status');
      const visible = currentLeadFilter === 'all'
        ? rowStatus !== 'archivado'
        : rowStatus === currentLeadFilter;
      row.style.display = visible ? '' : 'none';
    });
  }

  function updateLeadCounts() {
    const rows = Array.from(document.querySelectorAll('.lead-table-row'));
    const archived = rows.filter(r => r.getAttribute('data-status') === 'archivado').length;
    const activeEl = document.getElementById('leadCountActive');
    const archivedEl = document.getElementById('leadCountArchived');
    if (activeEl) activeEl.textContent = rows.length - archived;
    if (archivedEl) archivedEl.textContent = archived;
  }

  function syncLeadRow(leadId, status) {
    const row = document.getElementById(`lead-row-${leadId}`);
    if (!row) return;
    row.setAttribute('data-status', status);
    const isArchived = status === 'archivado';
    const archiveBtn = row.querySelector('[data-archive-lead]');
    const restoreBtn = row.querySelector('[data-unarchive-lead]');
    const deleteBtn = row.querySelector('[data-delete-lead]');
    if (archiveBtn) archiveBtn.style.display = isArchived ? 'none' : '';
    if (restoreBtn) restoreBtn.style.display = isArchived ? '' : 'none';
    if (deleteBtn) deleteBtn.style.display = isArchived ? '' : 'none';
    const stCfg = STATUS_CONFIG[status] || { label: status, class: 'badge-status-nuevo' };
    const badge = document.getElementById(`lead-status-badge-${leadId}`);
    if (badge) {
      badge.textContent = stCfg.label;
      badge.className = `badge-tag ${stCfg.class}`;
    }
    applyLeadFilter();
    updateLeadCounts();
  }

  filterBtns.forEach(btn => {
    btn.addEventListener('click', () => {
      filterBtns.forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      currentLeadFilter = btn.getAttribute('data-filter');
      applyLeadFilter();
    });
  });
  applyLeadFilter();
  updateLeadCounts();

  // Change lead status keeping its existing notes intact
  async function setLeadStatus(leadId, status) {
    const getRes = await fetch(`/api/leads/${leadId}`);
    if (!getRes.ok) throw new Error('No se pudo leer la consulta');
    const lead = await getRes.json();
    const res = await fetch(`/api/leads/${leadId}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ status, notes: lead.notes || '' })
    });
    if (!res.ok) throw new Error(await res.text());
    return lead.status;
  }

  document.addEventListener('click', async (e) => {
    const archiveBtn = e.target.closest('[data-archive-lead]');
    const restoreBtn = e.target.closest('[data-unarchive-lead]');
    const deleteBtn = e.target.closest('[data-delete-lead]');

    if (archiveBtn) {
      const leadId = archiveBtn.getAttribute('data-archive-lead');
      archiveBtn.disabled = true;
      try {
        const prev = await setLeadStatus(leadId, 'archivado');
        const row = document.getElementById(`lead-row-${leadId}`);
        if (row && prev && prev !== 'archivado') row.setAttribute('data-prev-status', prev);
        syncLeadRow(leadId, 'archivado');
      } catch (err) {
        alert('Error al archivar la consulta: ' + err.message);
      } finally {
        archiveBtn.disabled = false;
      }
      return;
    }

    if (restoreBtn) {
      const leadId = restoreBtn.getAttribute('data-unarchive-lead');
      const row = document.getElementById(`lead-row-${leadId}`);
      const target = (row && row.getAttribute('data-prev-status')) || 'nuevo';
      restoreBtn.disabled = true;
      try {
        await setLeadStatus(leadId, target);
        syncLeadRow(leadId, target);
      } catch (err) {
        alert('Error al restaurar la consulta: ' + err.message);
      } finally {
        restoreBtn.disabled = false;
      }
      return;
    }

    if (deleteBtn) {
      const leadId = deleteBtn.getAttribute('data-delete-lead');
      const row = document.getElementById(`lead-row-${leadId}`);
      const name = row ? (row.querySelector('td:nth-child(2) div')?.textContent || '').trim() : '';
      if (!confirm(`⚠️ ¿Eliminar DEFINITIVAMENTE la consulta #${leadId}${name ? ' de ' + name : ''}?\n\nEsta acción NO se puede deshacer.`)) return;
      deleteBtn.disabled = true;
      try {
        const res = await fetch(`/api/leads/${leadId}`, { method: 'DELETE' });
        if (res.ok) {
          if (row) {
            row.classList.add('row-removing');
            setTimeout(() => { row.remove(); updateLeadCounts(); }, 350);
          }
        } else {
          alert('No se pudo eliminar: ' + (await res.text()));
          deleteBtn.disabled = false;
        }
      } catch (err) {
        alert('Error de conexión al eliminar');
        deleteBtn.disabled = false;
      }
    }
  });

  // Lead Detail Modal
  const leadDetailModal = document.getElementById('leadDetailModal');
  let currentLeadId = null;

  async function openLeadDetails(leadId) {
    currentLeadId = leadId;
    if (!leadDetailModal) return;

    leadDetailModal.classList.add('active');
    document.body.style.overflow = 'hidden';

    // Temporary loading state
    document.getElementById('leadModalTitle').textContent = `Solicitud #${leadId}`;
    document.getElementById('leadModalClientName').textContent = 'Cargando datos...';
    document.getElementById('leadModalClientMeta').textContent = 'Consultando base de datos...';
    document.getElementById('leadModalMessage').textContent = '...';
    const statusSpan = document.getElementById('leadModalSaveStatus');
    if (statusSpan) statusSpan.style.display = 'none';

    try {
      const res = await fetch(`/api/leads/${leadId}`);
      if (!res.ok) throw new Error('No se pudo obtener la solicitud');
      const lead = await res.json();

      document.getElementById('leadModalTitle').textContent = `Solicitud #${lead.id}`;
      document.getElementById('leadModalClientName').textContent = lead.name || 'Sin nombre';
      document.getElementById('leadModalClientMeta').textContent = `Recibido el ${formatLeadDate(lead.created_at)}`;

      // Header Badge
      const stCfg = STATUS_CONFIG[lead.status] || { label: lead.status || 'Nuevo', class: 'badge-status-nuevo' };
      const headerBadge = document.getElementById('leadModalHeaderBadge');
      if (headerBadge) {
        headerBadge.textContent = stCfg.label;
        headerBadge.className = `badge-tag ${stCfg.class}`;
      }

      // Phone & WhatsApp
      const cleanPhone = (lead.phone || '').replace(/[^0-9]/g, '');
      document.getElementById('leadModalPhone').textContent = lead.phone || '-';
      const waBtn = document.getElementById('leadModalWaBtn');
      if (waBtn) {
        if (cleanPhone) {
          const carTitle = lead.car_title || 'un vehículo';
          const greeting = encodeURIComponent(`Hola ${lead.name || ''}, te contacto de Escocar Autos por tu consulta sobre ${carTitle}`);
          waBtn.href = `https://wa.me/${cleanPhone}?text=${greeting}`;
          waBtn.style.display = 'inline-flex';
        } else {
          waBtn.style.display = 'none';
        }
      }

      // Email
      document.getElementById('leadModalEmail').textContent = lead.email || 'No proporcionado';
      const mailBtn = document.getElementById('leadModalMailBtn');
      if (mailBtn) {
        if (lead.email) {
          mailBtn.href = `mailto:${lead.email}?subject=${encodeURIComponent('Consulta Escocar Autos - ' + (lead.car_title || ''))}`;
          mailBtn.style.display = 'inline-flex';
        } else {
          mailBtn.style.display = 'none';
        }
      }

      // Vehicle
      const carContainer = document.getElementById('leadModalCar');
      if (carContainer) {
        if (lead.car_id) {
          carContainer.innerHTML = `<a href="/auto/${lead.car_id}" target="_blank" style="color: var(--accent-gold); font-weight: 700; text-decoration: none;">${lead.car_title} ↗</a>`;
        } else {
          carContainer.innerHTML = `<span style="color: var(--text-muted); font-weight: 600;">${lead.car_title || 'Consulta General'}</span>`;
        }
      }

      // Visitor Insights
      const activityEl = document.getElementById('leadModalActivity');
      if (activityEl) {
        const visits = lead.visitor_visits || 1;
        const device = lead.visitor_device === 'mobile' ? '📱 Celular' : '💻 Computadora';
        activityEl.textContent = `${visits} ${visits === 1 ? 'visita' : 'visitas'} (${device})`;
      }

      // Cars seen list
      const carsSeenBox = document.getElementById('leadModalCarsSeenContainer');
      const carsSeenList = document.getElementById('leadModalCarsSeenList');
      if (carsSeenBox && carsSeenList) {
        if (lead.visitor_cars_seen && lead.visitor_cars_seen.length > 0) {
          carsSeenList.innerHTML = lead.visitor_cars_seen.map(c => `<span class="lead-cars-seen-tag">🚗 ${c}</span>`).join('');
          carsSeenBox.style.display = 'block';
        } else {
          carsSeenBox.style.display = 'none';
        }
      }

      // Message
      document.getElementById('leadModalMessage').textContent = lead.message || '(Sin mensaje adicional escrito en el formulario)';

      // Form Controls
      const statusSelect = document.getElementById('leadStatusSelect');
      if (statusSelect) statusSelect.value = lead.status || 'nuevo';

      const notesTextarea = document.getElementById('leadNotesTextarea');
      if (notesTextarea) notesTextarea.value = lead.notes || '';

      const lastUp = document.getElementById('leadLastUpdated');
      if (lastUp) {
        if (lead.updated_at && lead.updated_at !== lead.created_at) {
          lastUp.textContent = `Modificado: ${formatLeadDate(lead.updated_at)}`;
        } else {
          lastUp.textContent = '';
        }
      }
    } catch (err) {
      alert('Error al abrir la consulta: ' + err.message);
      closeLeadDetails();
    }
  }

  function closeLeadDetails() {
    if (leadDetailModal) leadDetailModal.classList.remove('active');
    document.body.style.overflow = '';
    currentLeadId = null;
  }

  // Open button click event
  document.addEventListener('click', (e) => {
    const openBtn = e.target.closest('.open-lead-detail-btn');
    if (openBtn) {
      const leadId = openBtn.getAttribute('data-lead-id');
      if (leadId) openLeadDetails(leadId);
    }
  });

  // Modal Close buttons
  const closeBtn = document.getElementById('closeLeadDetailModal');
  const cancelBtn = document.getElementById('cancelLeadDetailBtn');
  if (closeBtn) closeBtn.addEventListener('click', closeLeadDetails);
  if (cancelBtn) cancelBtn.addEventListener('click', closeLeadDetails);
  if (leadDetailModal) {
    leadDetailModal.addEventListener('click', (e) => {
      if (e.target === leadDetailModal) closeLeadDetails();
    });
  }
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && leadDetailModal && leadDetailModal.classList.contains('active')) {
      closeLeadDetails();
    }
  });

  // Save Lead Changes (Status & Notes)
  const saveLeadBtn = document.getElementById('saveLeadDetailBtn');
  if (saveLeadBtn) {
    saveLeadBtn.addEventListener('click', async () => {
      if (!currentLeadId) return;

      const newStatus = document.getElementById('leadStatusSelect').value;
      const newNotes = document.getElementById('leadNotesTextarea').value;
      const statusSpan = document.getElementById('leadModalSaveStatus');

      saveLeadBtn.disabled = true;
      const origText = saveLeadBtn.textContent;
      saveLeadBtn.textContent = 'Guardando...';
      if (statusSpan) statusSpan.style.display = 'none';

      try {
        const res = await fetch(`/api/leads/${currentLeadId}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ status: newStatus, notes: newNotes })
        });

        if (res.ok) {
          const updated = await res.json();

          // Show status feedback
          if (statusSpan) {
            statusSpan.style.display = 'inline';
            statusSpan.style.color = '#34d399';
            statusSpan.textContent = '✓ Guardado exitosamente';
          }

          // Update header badge
          const stCfg = STATUS_CONFIG[newStatus] || { label: newStatus, class: 'badge-status-nuevo' };
          const headerBadge = document.getElementById('leadModalHeaderBadge');
          if (headerBadge) {
            headerBadge.textContent = stCfg.label;
            headerBadge.className = `badge-tag ${stCfg.class}`;
          }

          // Update row in table
          const row = document.getElementById(`lead-row-${currentLeadId}`);
          if (row) {
            syncLeadRow(currentLeadId, newStatus);
            const notesPreview = document.getElementById(`lead-notes-preview-${currentLeadId}`);
            if (notesPreview) {
              notesPreview.setAttribute('title', newNotes);
              if (newNotes && newNotes.trim() !== '') {
                notesPreview.innerHTML = `<span style="color: #f1f5f9;">📝 ${escapeHtml(newNotes)}</span>`;
              } else {
                notesPreview.innerHTML = `<span style="color: var(--text-muted); font-style: italic; font-size: 0.8rem;">Sin notas</span>`;
              }
            }
          }

          // Update last updated timestamp
          const lastUp = document.getElementById('leadLastUpdated');
          if (lastUp && updated.updated_at) {
            lastUp.textContent = `Modificado: ${formatLeadDate(updated.updated_at)}`;
          }

          setTimeout(() => {
            if (statusSpan) statusSpan.style.display = 'none';
          }, 3000);
        } else {
          const errText = await res.text();
          alert('Error al guardar cambios: ' + errText);
        }
      } catch (err) {
        alert('Error de conexión al guardar cambios de la consulta');
      } finally {
        saveLeadBtn.disabled = false;
        saveLeadBtn.textContent = origText;
      }
    });
  }
});

