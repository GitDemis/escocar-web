/**
 * Escocar Autos — Client-Side Image Optimization Web Worker
 * Encodes stripped RGBA pixel buffers into Full-Res & Thumbnail WebP and JPEG XL (JXL)
 * using WebAssembly (libwebp + libjxl) off the main UI thread.
 */
import jxlEncoderFactory from './codecs/jxl_enc.js';
import webpEncoderFactory from './codecs/webp_enc.js';

let jxlModulePromise = null;
let webpModulePromise = null;

function getJxlModule() {
  if (!jxlModulePromise) {
    jxlModulePromise = jxlEncoderFactory({ noInitialRun: true });
  }
  return jxlModulePromise;
}

function getWebpModule() {
  if (!webpModulePromise) {
    webpModulePromise = webpEncoderFactory({ noInitialRun: true });
  }
  return webpModulePromise;
}

const DEFAULT_JXL_OPTIONS = {
  effort: 3,
  quality: 90,
  progressive: false,
  epf: -1,
  lossyPalette: false,
  decodingSpeedTier: 0,
  photonNoiseIso: 0,
  lossyModular: false,
  lossless: false,
};

const DEFAULT_WEBP_OPTIONS = {
  quality: 90,
  target_size: 0,
  target_PSNR: 0,
  method: 4,
  sns_strength: 50,
  filter_strength: 60,
  filter_sharpness: 0,
  filter_type: 1,
  partitions: 0,
  segments: 4,
  pass: 1,
  show_compressed: 0,
  preprocessing: 0,
  autofilter: 0,
  partition_limit: 0,
  alpha_compression: 1,
  alpha_filtering: 1,
  alpha_quality: 100,
  lossless: 0,
  exact: 0,
  image_hint: 0,
  emulate_jpeg_size: 0,
  thread_level: 0,
  low_memory: 0,
  near_lossless: 100,
  use_delta_palette: 0,
  use_sharp_yuv: 0,
};

function toStandaloneBuffer(view) {
  if (!view) {
    throw new Error('WASM encoder returned empty buffer');
  }
  const copy = new Uint8Array(view.byteLength);
  copy.set(new Uint8Array(view.buffer, view.byteOffset, view.byteLength));
  return copy.buffer;
}

self.onmessage = async (event) => {
  const {
    id,
    fullBuffer,
    fullWidth,
    fullHeight,
    thumbBuffer,
    thumbWidth,
    thumbHeight,
  } = event.data;

  try {
    const [jxlMod, webpMod] = await Promise.all([getJxlModule(), getWebpModule()]);

    const fullRgba = new Uint8ClampedArray(fullBuffer);
    const thumbRgba = new Uint8ClampedArray(thumbBuffer);

    // 1. Encode Full-Resolution WebP (100% original pixel count, EXIF stripped)
    const fullWebpView = webpMod.encode(fullRgba, fullWidth, fullHeight, {
      ...DEFAULT_WEBP_OPTIONS,
      quality: 90,
    });
    const fullWebpOut = toStandaloneBuffer(fullWebpView);

    // 2. Encode Thumbnail WebP (<= 720x540, EXIF stripped)
    const thumbWebpView = webpMod.encode(thumbRgba, thumbWidth, thumbHeight, {
      ...DEFAULT_WEBP_OPTIONS,
      quality: 80,
    });
    const thumbWebpOut = toStandaloneBuffer(thumbWebpView);

    // 3. Encode Full-Resolution JPEG XL (100% original pixel count, EXIF stripped)
    const fullJxlView = jxlMod.encode(fullRgba, fullWidth, fullHeight, {
      ...DEFAULT_JXL_OPTIONS,
      quality: 90,
      effort: 3,
    });
    const fullJxlOut = toStandaloneBuffer(fullJxlView);

    // 4. Encode Thumbnail JPEG XL (<= 720x540, EXIF stripped)
    const thumbJxlView = jxlMod.encode(thumbRgba, thumbWidth, thumbHeight, {
      ...DEFAULT_JXL_OPTIONS,
      quality: 80,
      effort: 3,
    });
    const thumbJxlOut = toStandaloneBuffer(thumbJxlView);

    self.postMessage(
      {
        id,
        ok: true,
        fullWebpBuffer: fullWebpOut,
        thumbWebpBuffer: thumbWebpOut,
        fullJxlBuffer: fullJxlOut,
        thumbJxlBuffer: thumbJxlOut,
      },
      [fullWebpOut, thumbWebpOut, fullJxlOut, thumbJxlOut]
    );
  } catch (err) {
    self.postMessage({
      id,
      ok: false,
      error: err && err.message ? err.message : String(err),
    });
  }
};
