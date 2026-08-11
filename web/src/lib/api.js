let csrfToken = "";

export function setCSRFToken(value) { csrfToken = value || ""; }

export async function api(path, options = {}) {
	const isForm = options.body instanceof FormData;
  const response = await fetch(path, {
	...options,
	credentials: "same-origin",
    headers: { ...(!isForm ? { "Content-Type": "application/json" } : {}), ...(csrfToken && !["GET", "HEAD"].includes(options.method || "GET") ? { "X-CSRF-Token": csrfToken } : {}), ...(options.headers || {}) },
  });
  const raw = response.status === 204 ? "" : await response.text();
  let body = null;
  if (raw) {
    try {
      body = JSON.parse(raw);
    } catch {
      body = { message: raw };
    }
  }
	if (response.status === 401) window.dispatchEvent(new CustomEvent("meerkit:unauthorized"));
  if (!response.ok) throw new Error(body?.message || "请求失败");
  return body;
}

export async function apiText(path, options = {}) {
	const response = await fetch(path, { ...options, credentials: "same-origin" });
	const raw = response.status === 204 ? "" : await response.text();
	if (response.status === 401) window.dispatchEvent(new CustomEvent("meerkit:unauthorized"));
	if (!response.ok) {
		let message = raw || "请求失败";
		try { message = JSON.parse(raw)?.message || message; } catch { /* Keep the text response. */ }
		throw new Error(message);
	}
	return raw;
}

export async function apiBlob(path, options = {}) {
	const response = await fetch(path, {
		...options,
		credentials: "same-origin",
		headers: { "Content-Type": "application/json", ...(csrfToken && !["GET", "HEAD"].includes(options.method || "GET") ? { "X-CSRF-Token": csrfToken } : {}), ...(options.headers || {}) }
	});
	if (response.status === 401) window.dispatchEvent(new CustomEvent("meerkit:unauthorized"));
	if (!response.ok) {
		const raw = await response.text();
		let message = raw || "请求失败";
		try { message = JSON.parse(raw)?.message || message; } catch { /* Keep the text response. */ }
		throw new Error(message);
	}
	const disposition = response.headers.get("Content-Disposition") || "";
	const filename = disposition.match(/filename="?([^";]+)"?/i)?.[1] || "meerkit-config.zip";
	let summary = null;
	try { summary = JSON.parse(response.headers.get("X-Meerkit-Export-Summary") || "null"); } catch { /* Older servers do not provide an export summary. */ }
	return { blob: await response.blob(), filename, summary };
}
