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

export function buildAgentInstallCommand(url: string, fallbackUrl?: string, scriptBase64?: string) {
  if (scriptBase64?.trim()) {
    const marker = shellQuote(INSTALL_MAGIC_MARKER)
    return [
      'enc=$(mktemp)',
      'tmp=$(mktemp)',
      `printf %s ${shellQuote(scriptBase64.trim())} > "$enc"`,
      '(base64 -d < "$enc" > "$tmp" 2>/dev/null || base64 -D < "$enc" > "$tmp")',
      `{ grep -q ${marker} "$tmp" || { echo 'BackupX embedded installer is invalid.' >&2; head -5 "$tmp" >&2; false; }; }`,
      runScriptCommand('"$tmp"'),
    ].join(' && ') + '; rc=$?; rm -f "$enc" "$tmp"; test $rc -eq 0'
  }

  const primary = url.trim()
  const fallback = (fallbackUrl || legacyInstallUrl(primary)).trim()
  const urls = fallback && fallback !== primary ? [primary, fallback] : [primary]
  const marker = shellQuote(INSTALL_MAGIC_MARKER)
  const fetchScript = urls.length > 1
    ? `(curl -fsSL ${shellQuote(urls[0])} -o "$tmp" && grep -q ${marker} "$tmp" || curl -fsSL ${shellQuote(urls[1])} -o "$tmp")`
    : `(curl -fsSL ${shellQuote(urls[0])} -o "$tmp" && grep -q ${marker} "$tmp")`

  return [
    'tmp=$(mktemp)',
    fetchScript,
    `{ grep -q ${marker} "$tmp" || { echo 'BackupX install endpoint returned non-script content; check reverse proxy /api/install or /install forwarding.' >&2; head -5 "$tmp" >&2; false; }; }`,
    runScriptCommand('"$tmp"'),
  ].join(' && ') + '; rc=$?; rm -f "$tmp"; test $rc -eq 0'
}

export function buildAgentDownloadCommand(url: string, fallbackUrl?: string, scriptBase64?: string) {
  if (scriptBase64?.trim()) {
    const marker = shellQuote(INSTALL_MAGIC_MARKER)
    return [
      `printf %s ${shellQuote(scriptBase64.trim())} > /tmp/bx-agent-install.b64`,
      '(base64 -d < /tmp/bx-agent-install.b64 > /tmp/bx-agent-install.sh 2>/dev/null || base64 -D < /tmp/bx-agent-install.b64 > /tmp/bx-agent-install.sh)',
      `{ grep -q ${marker} /tmp/bx-agent-install.sh || { echo 'BackupX embedded installer is invalid.' >&2; head -5 /tmp/bx-agent-install.sh >&2; false; }; }`,
      runScriptCommand('/tmp/bx-agent-install.sh'),
    ].join(' && ')
  }

  const primary = url.trim()
  const fallback = (fallbackUrl || legacyInstallUrl(primary)).trim()
  const marker = shellQuote(INSTALL_MAGIC_MARKER)
  const fetchScript = fallback && fallback !== primary
    ? `(curl -fsSL ${shellQuote(primary)} -o /tmp/bx-agent-install.sh && grep -q ${marker} /tmp/bx-agent-install.sh || curl -fsSL ${shellQuote(fallback)} -o /tmp/bx-agent-install.sh)`
    : `(curl -fsSL ${shellQuote(primary)} -o /tmp/bx-agent-install.sh && grep -q ${marker} /tmp/bx-agent-install.sh)`

  return [
    fetchScript,
    `{ grep -q ${marker} /tmp/bx-agent-install.sh || { echo 'BackupX install endpoint returned non-script content; check reverse proxy /api/install or /install forwarding.' >&2; head -5 /tmp/bx-agent-install.sh >&2; false; }; }`,
    runScriptCommand('/tmp/bx-agent-install.sh'),
  ].join(' && ')
}
