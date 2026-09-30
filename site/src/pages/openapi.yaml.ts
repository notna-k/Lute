// Serves the spec the API reference is built from, for download and for API clients.
import raw from '../../../api/openapi.yaml?raw';

export function GET() {
	return new Response(raw, { headers: { 'Content-Type': 'application/yaml; charset=utf-8' } });
}
