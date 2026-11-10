// Isolated iframe API only. Rendering/format dispatch lives outside this string
// and outside the sandbox, so future formats don't require CSP changes.
export const SCREENSHOT_API = `
  var __screenshotId = 0;
  var __screenshots = Object.create(null);
  function __closeScreenshots() {
    Object.keys(__screenshots).forEach(function(id) {
      var pending = __screenshots[id];
      clearTimeout(pending.timer);
      pending.reject(new Error('sandbox finished'));
      delete __screenshots[id];
    });
  }
  async function screenshot(source, options) {
    if (window.sandbox._done) throw new Error('sandbox finished');
    options = options || {};
    var id = ++__screenshotId;
    if (id > 6) throw new Error('screenshot: maximum 6 requests per run');
    var filename = options.filename;
    if (typeof source === 'string') {
      filename = __fileNames[source] || source;
      source = window.sandbox.readFile(source);
    }
    var buf = typeof Blob !== 'undefined' && source instanceof Blob ? await source.arrayBuffer() : toBuf(source, 'screenshot');
    if (!buf.byteLength || buf.byteLength > 50 * 1024 * 1024) throw new Error('screenshot: input must be between 1 byte and 50MB');
    if (typeof filename !== 'string' || !filename || filename.length > 255) throw new Error('screenshot: filename is required for bytes or Blob input');
    if (window.sandbox._done) throw new Error('sandbox finished');
    var copy = buf.slice(0);
    return new Promise(function(resolve, reject) {
      var timer = setTimeout(function() {
        delete __screenshots[id];
        reject(new Error('screenshot timed out'));
      }, 60000);
      __screenshots[id] = {resolve: resolve, reject: reject, timer: timer};
      try {
        send({type: '__sandbox_screenshot', requestId: id, data: copy, filename: filename,
          options: {page: options.page, width: options.width, height: options.height, range: options.range}}, [copy]);
      } catch (error) {
        clearTimeout(timer);
        delete __screenshots[id];
        reject(error);
      }
    });
  }
  window.addEventListener('message', function(event) {
    if (event.source !== window.parent) return;
    var data = event.data;
    if (!data || data.type !== '__sandbox_screenshot_result') return;
    var pending = __screenshots[data.requestId];
    if (!pending) return;
    clearTimeout(pending.timer);
    delete __screenshots[data.requestId];
    if (data.error) pending.reject(new Error(String(data.error)));
    else if (data.data instanceof ArrayBuffer) pending.resolve(new Blob([data.data], {type: 'image/png'}));
    else pending.reject(new Error('invalid screenshot response'));
  });
`;
