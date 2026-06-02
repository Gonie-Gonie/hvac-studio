export async function api(path, options = {}) {
  const response = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  const body = await response.json();
  if (!response.ok || body.ok === false) {
    const error = new Error(body.message || `Request failed: ${path}`);
    error.body = body;
    throw error;
  }
  return body;
}
