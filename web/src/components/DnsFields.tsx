import type { DnsProvider } from "../hooks/useDnsProviders";
import { useI18n } from "../i18n";
import { IdField, Select, Textarea } from "./ui/form";

/** DNS provider select + credentials textarea prefilled with the provider template (NPM-style). */
export function DnsFields({ providers, provider, creds, onChange, error }: {
  providers: DnsProvider[]; provider: string; creds: string; onChange: (p: { provider: string; creds: string }) => void; error?: string;
}) {
  const { t } = useI18n();
  return (
    <>
      <IdField label={t("dnsProvider")}>
        {(id) => (
          <Select id={id} value={provider} onChange={(e) => onChange({ provider: e.target.value, creds: providers.find((p) => p.id === e.target.value)?.credentials_template ?? "" })}>
            {providers.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
          </Select>
        )}
      </IdField>
      <IdField label={t("dnsCredentials")} required hint={t("dnsCredentialsHint")} error={error}>
        {(id) => <Textarea id={id} rows={4} spellCheck={false} autoComplete="off" className="font-mono text-xs leading-5" invalid={!!error}
          value={creds} onChange={(e) => onChange({ provider, creds: e.target.value })} />}
      </IdField>
    </>
  );
}
