'use strict';
const REPOSITORY = 'ArthWarrior3201/GeoCheats';
const API = `https://api.github.com/repos/${REPOSITORY}/releases/latest`;
let attempt = 0;
let currentController;
const loading = document.getElementById('loading');
const downloads = document.getElementById('downloads');
const button = document.getElementById('download');
function selectAssets(assets) {
  const uploaded = assets.filter(a => a.state === 'uploaded' && a.browser_download_url);
  const one = test => uploaded.find(a => test(a.name.toLowerCase()));
  return {
    Windows: one(n => /windows.*\.exe$/.test(n) && !/installer|setup|dash-cli/.test(n)),
    macOS: one(n => /(?:macos|darwin).*\.zip$/.test(n)),
    Linux: one(n => /linux.*\.zip$/.test(n))
  };
}
function safeAssetURL(value) {
  try { const u = new URL(value); return u.protocol === 'https:' && u.hostname === 'github.com' && u.pathname.startsWith(`/${REPOSITORY}/releases/download/`) ? u.href : null; } catch { return null; }
}
function renderRelease(release, error) {
  document.getElementById('error').textContent = error || '';
  document.getElementById('release-name').textContent = release ? release.name || release.tag_name : 'Latest release unavailable';
  const notes = document.getElementById('release-link');
  notes.href = `https://github.com/${REPOSITORY}/releases`;
  if (release && typeof release.tag_name === 'string') notes.href += `/tag/${encodeURIComponent(release.tag_name)}`;
  const assets = selectAssets(release && Array.isArray(release.assets) ? release.assets : []);
  const panel = document.getElementById('platforms'); panel.replaceChildren();
  for (const [platform, hint] of [['Windows', 'Portable .exe'], ['macOS', 'Application .zip'], ['Linux', '.deb + .rpm .zip']]) {
    const a = document.createElement('a'); a.className = 'os';
    const label = document.createElement('span'); label.textContent = platform;
    const info = document.createElement('small'); const asset = assets[platform]; const href = asset && safeAssetURL(asset.browser_download_url);
    if (href) { a.href = href; a.setAttribute('download', asset.name); info.textContent = hint + ' ↓'; }
    else { a.setAttribute('aria-disabled', 'true'); info.textContent = 'Not uploaded'; }
    a.append(label, info); panel.append(a);
  }
}
async function checkLatest() {
  const id = ++attempt;
  if (currentController) currentController.abort();
  currentController = new AbortController();
  const controller = currentController;
  button.disabled = true;
  if (downloads.open) downloads.close();
  loading.showModal();
  let release = null, failure = '';
  // No storage, no retained release, no version fallback: every click starts a new request.
  const timer = setTimeout(() => controller.abort(), 4800);
  const request = (async () => {
    try {
      const response = await fetch(API + '?fresh=' + Date.now(), {cache: 'no-store', headers: {Accept: 'application/vnd.github+json'}, signal: controller.signal});
      if (!response.ok) throw new Error(response.status === 404 ? 'No published stable release yet.' : `GitHub returned ${response.status}. Please try again.`);
      release = await response.json();
      if (release.draft || release.prerelease || !release.tag_name) throw new Error('GitHub did not return a published stable release.');
    } catch (error) { release = null; failure = error.name === 'AbortError' ? 'GitHub did not respond within the loading window. Click Download to retry.' : error.message; }
    finally { clearTimeout(timer); }
  })();
  await Promise.all([request, new Promise(resolve => setTimeout(resolve, 5000))]);
  if (id !== attempt) return;
  loading.close(); renderRelease(release, failure); downloads.showModal(); button.disabled = false;
}
function cancelLoading() { ++attempt; if (currentController) currentController.abort(); loading.close(); button.disabled = false; }
button.addEventListener('click', checkLatest);
loading.addEventListener('cancel', event => { event.preventDefault(); cancelLoading(); });
document.querySelectorAll('[data-close]').forEach(b => b.addEventListener('click', () => b.dataset.close === 'loading' ? cancelLoading() : document.getElementById(b.dataset.close).close()));
