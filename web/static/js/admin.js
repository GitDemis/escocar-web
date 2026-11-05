/**
 * Escocar Autos - Admin Dashboard Scripts
 */
document.addEventListener('DOMContentLoaded', () => {
  'use strict';

  // Auto-redirect to Google OAuth login if any admin API request returns 401 (expired session)
  const nativeFetch = window.fetch.bind(window);
  window.fetch = async function(...args) {
    const res = await nativeFetch(...args);
    if (res && res.status === 401) {
      window.location.href = '/auth/google/login';
    }
    return res;
  };

  // 1. Tab Navigation (Left Lateral Dock)
  const tabs = document.querySelectorAll('.admin-tab');
  const panels = document.querySelectorAll('.tab-content');

  function activateAdminTab(target) {
    if (!target) return;
    const panel = document.getElementById(target);
    if (!panel) return;
    tabs.forEach(t => t.classList.toggle('active', t.getAttribute('data-tab') === target));
    panels.forEach(p => p.classList.toggle('active', p.id === target));
    try {
      sessionStorage.setItem('escocar_admin_active_tab', target);
    } catch (e) {}
  }

  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      const target = tab.getAttribute('data-tab');
      activateAdminTab(target);
    });
  });

  try {
    const savedTab = sessionStorage.getItem('escocar_admin_active_tab');
    if (savedTab && document.getElementById(savedTab)) {
      activateAdminTab(savedTab);
    }
  } catch (e) {}

  // Helpers for file uploads & 100% Client-Side EXIF Strip + WebP/JXL WASM Pipeline
  function isImageFile(file) {
    return file && file.type && (file.type.startsWith('image/jpeg') || file.type.startsWith('image/png') || file.type.startsWith('image/webp'));
  }

  let imageWorker = null;
  let workerJobSeq = 0;
  const workerPendingJobs = new Map();
  const processedFileCache = new Map();
  let clientEncodeQueue = Promise.resolve();
  let fileUidCounter = 0;

  function getFileUid(file) {
    if (!file._escocarUid) {
      file._escocarUid = 'img_' + (++fileUidCounter) + '_' + Date.now();
    }
    return file._escocarUid;
  }

  function sanitizeBaseStem(filename) {
    const withoutExt = (filename || 'foto').replace(/\.[^/.]+$/, '');
    const cleaned = withoutExt.replace(/[^a-zA-Z0-9_-]/g, '_').replace(/^[_-]+|[_-]+$/g, '').slice(0, 20);
    return cleaned || 'img';
  }

  function getOrInitImageWorker() {
    if (imageWorker) return imageWorker;
    try {
      imageWorker = new Worker('/static/js/image-worker.js', { type: 'module' });
      imageWorker.onmessage = (e) => {
        const data = e.data || {};
        const pending = workerPendingJobs.get(data.id);
        if (!pending) return;
        workerPendingJobs.delete(data.id);
        if (data.ok) {
          pending.resolve(data);
        } else {
          pending.reject(new Error(data.error || 'Error en worker de imagen'));
        }
      };
      imageWorker.onerror = (err) => {
        console.warn('Error en Web Worker de imágenes, usando fallback Canvas:', err);
        workerPendingJobs.forEach((p) => p.reject(err));
        workerPendingJobs.clear();
      };
    } catch (e) {
      console.warn('No se pudo iniciar Web Worker de imágenes:', e);
      imageWorker = null;
    }
    return imageWorker;
  }

  async function decodeFileToImageSource(file) {
    if (typeof createImageBitmap === 'function') {
      try {
        return await createImageBitmap(file, { imageOrientation: 'from-image' });
      } catch (_) {
        return await createImageBitmap(file);
      }
    }
    return new Promise((resolve, reject) => {
      const img = new Image();
      const url = URL.createObjectURL(file);
      img.onload = () => {
        URL.revokeObjectURL(url);
        resolve(img);
      };
      img.onerror = (err) => {
        URL.revokeObjectURL(url);
        reject(err);
      };
      img.src = url;
    });
  }

  function canvasToBlob(canvas, type, quality) {
    return new Promise((resolve, reject) => {
      canvas.toBlob((blob) => {
        if (blob) resolve(blob);
        else reject(new Error('Fallo al exportar canvas'));
      }, type, quality);
    });
  }

  async function processSingleImageClientSide(file) {
    const stem = sanitizeBaseStem(file.name);
    const source = await decodeFileToImageSource(file);
    const fullWidth = source.width || source.naturalWidth;
    const fullHeight = source.height || source.naturalHeight;

    // 1. Draw full-resolution canvas (100% original pixel count, strips all EXIF/GPS metadata)
    const fullCanvas = document.createElement('canvas');
    fullCanvas.width = fullWidth;
    fullCanvas.height = fullHeight;
    const fullCtx = fullCanvas.getContext('2d', { willReadFrequently: true });
    fullCtx.drawImage(source, 0, 0, fullWidth, fullHeight);

    // 2. Draw thumbnail canvas (fits inside 720x540 preserving aspect ratio)
    const scale = Math.min(1, 720 / fullWidth, 540 / fullHeight);
    const thumbWidth = Math.max(1, Math.round(fullWidth * scale));
    const thumbHeight = Math.max(1, Math.round(fullHeight * scale));
    const thumbCanvas = document.createElement('canvas');
    thumbCanvas.width = thumbWidth;
    thumbCanvas.height = thumbHeight;
    const thumbCtx = thumbCanvas.getContext('2d', { willReadFrequently: true });
    thumbCtx.drawImage(source, 0, 0, thumbWidth, thumbHeight);

    if (typeof source.close === 'function') {
      try { source.close(); } catch (_) {}
    }

    const worker = getOrInitImageWorker();
    if (worker) {
      try {
        const fullImageData = fullCtx.getImageData(0, 0, fullWidth, fullHeight);
        const thumbImageData = thumbCtx.getImageData(0, 0, thumbWidth, thumbHeight);
        // Free canvas memory immediately before WASM encoding
        fullCanvas.width = 0; fullCanvas.height = 0;
        thumbCanvas.width = 0; thumbCanvas.height = 0;

        const jobId = ++workerJobSeq;
        const workerRes = await new Promise((resolve, reject) => {
          workerPendingJobs.set(jobId, { resolve, reject });
          worker.postMessage(
            {
              id: jobId,
              fullBuffer: fullImageData.data.buffer,
              fullWidth,
              fullHeight,
              thumbBuffer: thumbImageData.data.buffer,
              thumbWidth,
              thumbHeight,
            },
            [fullImageData.data.buffer, thumbImageData.data.buffer]
          );
        });

        return {
          status: 'done',
          fullWebpFile: new File([workerRes.fullWebpBuffer], `${stem}.webp`, { type: 'image/webp' }),
          thumbWebpFile: new File([workerRes.thumbWebpBuffer], `${stem}_thumb.webp`, { type: 'image/webp' }),
          fullJxlFile: new File([workerRes.fullJxlBuffer], `${stem}.jxl`, { type: 'image/jxl' }),
          thumbJxlFile: new File([workerRes.thumbJxlBuffer], `${stem}_thumb.jxl`, { type: 'image/jxl' }),
        };
      } catch (workerErr) {
        console.warn('Fallback a Canvas nativo tras error en WASM worker:', workerErr);
      }
    }

    // Fallback: native Canvas WebP export (still 100% EXIF-free)
    const fullBlob = await canvasToBlob(fullCanvas, 'image/webp', 0.92);
    const thumbBlob = await canvasToBlob(thumbCanvas, 'image/webp', 0.80);
    fullCanvas.width = 0; fullCanvas.height = 0;
    thumbCanvas.width = 0; thumbCanvas.height = 0;

    return {
      status: 'done',
      fullWebpFile: new File([fullBlob], `${stem}.webp`, { type: fullBlob.type || 'image/webp' }),
      thumbWebpFile: new File([thumbBlob], `${stem}_thumb.webp`, { type: thumbBlob.type || 'image/webp' }),
      fullJxlFile: null,
      thumbJxlFile: null,
    };
  }

  function enqueueFilesForOptimization(files, onStatusChange) {
    files.forEach((file) => {
      const uid = getFileUid(file);
      if (processedFileCache.has(uid)) return;
      const record = {
        status: 'pending',
        fullWebpFile: null,
        thumbWebpFile: null,
        fullJxlFile: null,
        thumbJxlFile: null,
        promise: null,
      };
      record.promise = (clientEncodeQueue = clientEncodeQueue.then(async () => {
        record.status = 'processing';
        if (onStatusChange) onStatusChange();
        try {
          const res = await processSingleImageClientSide(file);
          record.status = 'done';
          record.fullWebpFile = res.fullWebpFile;
          record.thumbWebpFile = res.thumbWebpFile;
          record.fullJxlFile = res.fullJxlFile;
          record.thumbJxlFile = res.thumbJxlFile;
        } catch (err) {
          console.error('Error optimizando imagen en cliente:', err);
          record.status = 'error';
        }
        if (onStatusChange) onStatusChange();
        return record;
      }));
      processedFileCache.set(uid, record);
    });
  }

  async function appendOptimizedFilesToFormData(formData, files, submitBtn, labelPrefix) {
    formData.delete('images');
    formData.delete('images_thumb_webp');
    formData.delete('images_jxl');
    formData.delete('images_thumb_jxl');

    for (let i = 0; i < files.length; i++) {
      const file = files[i];
      const uid = getFileUid(file);
      if (!processedFileCache.has(uid)) {
        enqueueFilesForOptimization([file]);
      }
      const record = processedFileCache.get(uid);
      if (record && record.status !== 'done' && record.promise) {
        if (submitBtn) {
          submitBtn.textContent = `${labelPrefix} (${i + 1}/${files.length} WebP+JXL)...`;
        }
        await record.promise;
      }
      if (record && record.status === 'done' && record.fullWebpFile) {
        formData.append('images', record.fullWebpFile, record.fullWebpFile.name);
        if (record.thumbWebpFile) {
          formData.append('images_thumb_webp', record.thumbWebpFile, record.thumbWebpFile.name);
        }
        if (record.fullJxlFile) {
          formData.append('images_jxl', record.fullJxlFile, record.fullJxlFile.name);
        }
        if (record.thumbJxlFile) {
          formData.append('images_thumb_jxl', record.thumbJxlFile, record.thumbJxlFile.name);
        }
      } else {
        formData.append('images', file, file.name);
      }
    }
  }

  function getOptimizationBadgeHtml(file) {
    const record = processedFileCache.get(getFileUid(file));
    if (!record || record.status === 'pending' || record.status === 'processing') {
      return '<span class="img-opt-status" style="position:absolute;bottom:4px;left:4px;right:4px;background:rgba(15,23,42,0.82);color:#fbbf24;font-size:0.65rem;padding:2px 5px;border-radius:4px;text-align:center;">⏳ WebP+JXL...</span>';
    }
    if (record.status === 'done') {
      return '<span class="img-opt-status" style="position:absolute;bottom:4px;left:4px;right:4px;background:rgba(6,78,59,0.88);color:#6ee7b7;font-size:0.65rem;padding:2px 5px;border-radius:4px;text-align:center;">✓ WebP+JXL · Sin GPS</span>';
    }
    return '';
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
        ${getOptimizationBadgeHtml(file)}
      `;
      addCarPreviewGrid.appendChild(card);
    });
  }

  if (addCarDropzone && addCarFileInput) {
    addCarFileInput.addEventListener('change', (e) => {
      const selected = Array.from(e.target.files).filter(isImageFile);
      addCarFiles = addCarFiles.concat(selected);
      enqueueFilesForOptimization(selected, renderAddCarPreviews);
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
        enqueueFilesForOptimization(dropped, renderAddCarPreviews);
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
  const carStatusSelect = document.getElementById('carStatus');
  const submitNewCarBtn = document.getElementById('submitNewCarBtn');
  if (carStatusSelect && submitNewCarBtn) {
    const updateSubmitBtnText = () => {
      if (carStatusSelect.value === 'borrador') {
        submitNewCarBtn.textContent = '💾 Guardar como Borrador (Solo Administradores)';
      } else {
        submitNewCarBtn.textContent = '🚀 Guardar y Publicar en Catálogo';
      }
    };
    carStatusSelect.addEventListener('change', updateSubmitBtnText);
    updateSubmitBtnText();
  }

  if (addCarForm) {
    addCarForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const submitBtn = submitNewCarBtn || addCarForm.querySelector('button[type="submit"]');
      const origText = submitBtn.textContent;
      submitBtn.disabled = true;
      submitBtn.textContent = 'Optimizando fotos...';

      const formData = new FormData(addCarForm);
      await appendOptimizedFilesToFormData(formData, addCarFiles, submitBtn, 'Optimizando fotos');
      submitBtn.textContent = 'Subiendo vehículo...';

      try {
        const res = await fetch('/api/cars', {
          method: 'POST',
          body: formData
        });

        if (res.ok) {
          const isDraft = formData.get('status') === 'borrador';
          if (isDraft) {
            alert('¡Vehículo guardado exitosamente como BORRADOR!\n\nNo está publicado en la web ni visible para los clientes. Podés verlo y probarlo desde el panel o con el botón "Ver" exclusivo para administradores.');
          } else {
            alert('¡Vehículo agregado exitosamente al inventario y publicado en el catálogo!');
          }
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
        ${getOptimizationBadgeHtml(file)}
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
      enqueueFilesForOptimization(selected, renderEditCarNewPreviews);
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
        enqueueFilesForOptimization(dropped, renderEditCarNewPreviews);
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
      saveBtn.textContent = 'Optimizando fotos...';

      const formData = new FormData(editCarForm);
      formData.set('existing_images', JSON.stringify(editCarExistingList));
      await appendOptimizedFilesToFormData(formData, editCarNewFiles, saveBtn, 'Optimizando fotos');
      saveBtn.textContent = 'Guardando cambios...';

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
          carContainer.innerHTML = `<a href="/auto/${encodeURIComponent(lead.car_id)}" target="_blank" style="color: var(--accent-gold); font-weight: 700; text-decoration: none;">${escapeHtml(lead.car_title || '')} ↗</a>`;
        } else {
          carContainer.innerHTML = `<span style="color: var(--text-muted); font-weight: 600;">${escapeHtml(lead.car_title || 'Consulta General')}</span>`;
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
          carsSeenList.innerHTML = lead.visitor_cars_seen.map(c => `<span class="lead-cars-seen-tag">🚗 ${escapeHtml(c)}</span>`).join('');
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

  // 7. Featured Cars Drag & Drop Reordering + Quick Toggle
  const featuredSortableGrid = document.getElementById('featuredSortableGrid');
  const saveFeaturedOrderBtn = document.getElementById('saveFeaturedOrderBtn');
  const featuredOrderStatus = document.getElementById('featuredOrderStatus');
  let draggedFeaturedCard = null;

  function refreshFeaturedOrderBadges() {
    if (!featuredSortableGrid) return;
    const cards = featuredSortableGrid.querySelectorAll('.featured-sort-card');
    cards.forEach((card, index) => {
      const badge = card.querySelector('[data-order-badge]');
      if (badge) {
        badge.textContent = `#${index + 1}`;
      }
    });
  }

  async function persistFeaturedOrder(showAlertOnError = true) {
    if (!featuredSortableGrid) return;
    const cards = Array.from(featuredSortableGrid.querySelectorAll('.featured-sort-card'));
    const carIds = cards
      .map(c => parseInt(c.getAttribute('data-car-id'), 10))
      .filter(n => !isNaN(n));

    if (saveFeaturedOrderBtn) {
      saveFeaturedOrderBtn.disabled = true;
      saveFeaturedOrderBtn.textContent = 'Guardando...';
    }

    try {
      const res = await fetch('/api/cars/featured-order', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ car_ids: carIds })
      });

      if (res.ok) {
        if (featuredOrderStatus) {
          featuredOrderStatus.style.display = 'inline-block';
          featuredOrderStatus.textContent = '✓ Orden guardado en inicio';
          setTimeout(() => {
            featuredOrderStatus.style.display = 'none';
          }, 3000);
        }
      } else if (showAlertOnError) {
        alert('Error al guardar el orden de destacados: ' + (await res.text()));
      }
    } catch (err) {
      if (showAlertOnError) {
        alert('Error de conexión al guardar el orden de destacados');
      }
    } finally {
      if (saveFeaturedOrderBtn) {
        saveFeaturedOrderBtn.disabled = false;
        saveFeaturedOrderBtn.textContent = '💾 Guardar Orden';
      }
    }
  }

  if (featuredSortableGrid) {
    featuredSortableGrid.addEventListener('dragstart', (e) => {
      const card = e.target.closest('.featured-sort-card');
      if (!card) return;
      draggedFeaturedCard = card;
      card.classList.add('dragging');
      if (e.dataTransfer) {
        e.dataTransfer.effectAllowed = 'move';
        e.dataTransfer.setData('text/plain', card.getAttribute('data-car-id') || '');
      }
    });

    featuredSortableGrid.addEventListener('dragend', () => {
      if (draggedFeaturedCard) {
        draggedFeaturedCard.classList.remove('dragging');
        draggedFeaturedCard = null;
      }
      featuredSortableGrid.querySelectorAll('.featured-sort-card').forEach(c => c.classList.remove('drag-over'));
      refreshFeaturedOrderBadges();
      persistFeaturedOrder(false);
    });

    featuredSortableGrid.addEventListener('dragover', (e) => {
      e.preventDefault();
      if (!draggedFeaturedCard) return;
      const targetCard = e.target.closest('.featured-sort-card');
      if (!targetCard || targetCard === draggedFeaturedCard) return;

      const rect = targetCard.getBoundingClientRect();
      const midX = rect.left + rect.width / 2;
      const midY = rect.top + rect.height / 2;
      const insertAfter = (e.clientY > midY + 25) || (Math.abs(e.clientY - midY) <= 25 && e.clientX > midX);

      if (insertAfter) {
        featuredSortableGrid.insertBefore(draggedFeaturedCard, targetCard.nextSibling);
      } else {
        featuredSortableGrid.insertBefore(draggedFeaturedCard, targetCard);
      }
      refreshFeaturedOrderBadges();
    });

    // Click arrows to move card left/right (great for touch / mobile)
    featuredSortableGrid.addEventListener('click', (e) => {
      const moveBtn = e.target.closest('.btn-move-card');
      if (!moveBtn) return;
      e.preventDefault();
      const card = moveBtn.closest('.featured-sort-card');
      if (!card) return;
      const dir = moveBtn.getAttribute('data-move-dir');
      if (dir === 'prev' && card.previousElementSibling) {
        featuredSortableGrid.insertBefore(card, card.previousElementSibling);
        refreshFeaturedOrderBadges();
        persistFeaturedOrder(false);
      } else if (dir === 'next' && card.nextElementSibling) {
        featuredSortableGrid.insertBefore(card.nextElementSibling, card);
        refreshFeaturedOrderBadges();
        persistFeaturedOrder(false);
      }
    });
  }

  if (saveFeaturedOrderBtn) {
    saveFeaturedOrderBtn.addEventListener('click', () => {
      persistFeaturedOrder(true);
    });
  }

  // Toggle Featured Status on any vehicle
  document.addEventListener('click', async (e) => {
    const toggleBtn = e.target.closest('[data-toggle-featured]');
    if (!toggleBtn) return;
    e.preventDefault();

    const carId = toggleBtn.getAttribute('data-toggle-featured');
    const currentlyFeatured = toggleBtn.getAttribute('data-featured') === 'true';
    const nextFeatured = !currentlyFeatured;
    toggleBtn.disabled = true;

    try {
      const res = await fetch(`/api/cars/${carId}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ featured: nextFeatured })
      });

      if (res.ok) {
        window.location.reload();
      } else {
        alert('No se pudo actualizar estado de destacado: ' + (await res.text()));
        toggleBtn.disabled = false;
      }
    } catch (err) {
      alert('Error de conexión al actualizar destacado');
      toggleBtn.disabled = false;
    }
  });
});

