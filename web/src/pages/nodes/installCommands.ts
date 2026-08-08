const INSTALL_MAGIC_MARKER = 'BACKUPX_AGENT_INSTALL_V1'

function shellQuote(value: string) {
  return `'${value.replace(/'/g, `'\\''`)}'`
}

function legacyInstallUrl(url: string) {
  return url.replace('/api/install/', '/install/')
}

function runScriptCommand(path: string) {
  return `if [ "$(id -u)" -eq 0 ]; then sh ${path}; else sudo sh ${path}; fi`
}

export interface InstallFetchOptions {
  proxyUrl?: string
  caCertFile?: string
}

function curlFetch(url: string, destination: string, options: InstallFetchOptions) {
  const args = ['curl', '-fsS']
  if (options.proxyUrl?.trim()) {
    args.push('--proxy', shellQuote(options.proxyUrl.trim()))
  }
  if (options.caCertFile?.trim()) {
    args.push('--cacert', shellQuote(options.caCertFile.trim()))
  }
  args.push(shellQuote(url), '-o', destination)
  return args.join(' ')
}

export function buildAgentInstallCommand(url: string, fallbackUrl?: string, options: InstallFetchOptions = {}) {
  const primary = url.trim()
  const fallback = (fallbackUrl || legacyInstallUrl(primary)).trim()
  const urls = fallback && fallback !== primary ? [primary, fallback] : [primary]
  const marker = shellQuote(INSTALL_MAGIC_MARKER)
  const fetchScript = urls.length > 1
    ? `(${curlFetch(urls[0], '"$tmp"', options)} && grep -q ${marker} "$tmp" || ${curlFetch(urls[1], '"$tmp"', options)})`
    : `(${curlFetch(urls[0], '"$tmp"', options)} && grep -q ${marker} "$tmp")`

  return [
    'umask 077',
    'tmp=$(mktemp)',
    fetchScript,
    `{ grep -q ${marker} "$tmp" || { echo 'BackupX install endpoint returned non-script content; check reverse proxy /api/install or /install forwarding.' >&2; head -5 "$tmp" >&2; false; }; }`,
    runScriptCommand('"$tmp"'),
  ].join(' && ') + '; rc=$?; rm -f "$tmp"; test $rc -eq 0'
}

export function buildAgentDownloadCommand(url: string, fallbackUrl?: string, options: InstallFetchOptions = {}) {
  const primary = url.trim()
  const fallback = (fallbackUrl || legacyInstallUrl(primary)).trim()
  const marker = shellQuote(INSTALL_MAGIC_MARKER)
  const fetchScript = fallback && fallback !== primary
    ? `(${curlFetch(primary, '"$tmp"', options)} && grep -q ${marker} "$tmp" || ${curlFetch(fallback, '"$tmp"', options)})`
    : `(${curlFetch(primary, '"$tmp"', options)} && grep -q ${marker} "$tmp")`

  return [
    'umask 077',
    'tmp=$(mktemp /tmp/bx-agent-install.XXXXXX)',
    fetchScript,
    `{ grep -q ${marker} "$tmp" || { echo 'BackupX install endpoint returned non-script content; check reverse proxy /api/install or /install forwarding.' >&2; head -5 "$tmp" >&2; false; }; }`,
    runScriptCommand('"$tmp"'),
  ].join(' && ') + '; rc=$?; rm -f "$tmp"; test $rc -eq 0'
}

export function buildEmbeddedAgentInstallCommand(scriptBase64: string) {
  const marker = shellQuote(INSTALL_MAGIC_MARKER)
  return [
    'umask 077',
    'enc=$(mktemp)',
    'tmp=$(mktemp)',
    `printf %s ${shellQuote(scriptBase64.trim())} > "$enc"`,
    '(base64 -d < "$enc" > "$tmp" 2>/dev/null || base64 -D < "$enc" > "$tmp")',
    `{ grep -q ${marker} "$tmp" || { echo 'BackupX embedded installer is invalid.' >&2; head -5 "$tmp" >&2; false; }; }`,
    runScriptCommand('"$tmp"'),
  ].join(' && ') + '; rc=$?; rm -f "$enc" "$tmp"; test $rc -eq 0'
}
