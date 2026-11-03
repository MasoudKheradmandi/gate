import { useResource } from "./useResource";

export interface DnsProvider { id: string; name: string; credentials_template: string }

export const FALLBACK_PROVIDERS: DnsProvider[] = [{
  id: "cloudflare", name: "Cloudflare",
  credentials_template: "# Cloudflare API token\ndns_cloudflare_api_token=0123456789abcdef0123456789abcdef01234567",
}];

/** DNS providers from the backend; falls back to a built-in Cloudflare entry when the endpoint is missing/empty. */
export function useDnsProviders(): DnsProvider[] {
  const r = useResource<DnsProvider[]>("/api/acme/dns-providers");
  return r.data?.length ? r.data : FALLBACK_PROVIDERS;
}
