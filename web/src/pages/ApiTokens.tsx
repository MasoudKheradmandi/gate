import { useMemo, useState } from "react";
import { Activity, Copy, KeySquare, Lock, Plus, ShieldAlert, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { api } from "../api";
import { ErrorBox, PageHeader, Toolbar, useSubmit } from "../components/common";
import { Button } from "../components/ui/button";
import { Badge, Callout, Card, CardHeader, ComingSoon, EmptyState, TableSkeleton } from "../components/ui/display";
import { FormSection, IdField, Input, SearchInput, Select, fieldErr } from "../components/ui/form";
import { ConfirmDialog, Modal, RowMenu, Sheet } from "../components/ui/overlay";
import { Table, THead, Td, Th, Tr } from "../components/ui/table";
import { useResource } from "../hooks/useResource";
import { useI18n } from "../i18n";
import { roleKey, useCan, type Role } from "../lib/rbac";
import { copyText, fmtDateTime, relTime } from "../lib/utils";

export interface ApiToken {
  id: number; name: string; prefix: string; role: Role; created_by: number; created_by_email: string;
  created_at: string; last_used_at: string | null; expires_at: string | null;
}
type Created = ApiToken & { token: string };

const ROLES: Role[] = ["admin", "operator", "viewer"];
const RANK: Record<Role, number> = { viewer: 1, operator: 2, admin: 3 };
const roleTone = { admin: "brand", operator: "info", viewer: "neutral" } as const;
const EXPIRY = [
  { days: 0, key: "tokExpNever" }, { days: 30, key: "tokExp30" }, { days: 90, key: "tokExp90" }, { days: 365, key: "tokExp365" },
] as const;

interface Form { name: string; role: Role; expires_days: number }
const blank: Form = { name: "", role: "viewer", expires_days: 90 };

function scrapeConfig(token: string): string {
  const host = typeof window !== "undefined" ? window.location.host : "corvus.example.com:9443";
  return [
    "scrape_configs:",
    "  - job_name: corvus",
    "    scheme: https",
    "    metrics_path: /metrics",
    "    authorization:",
    "      type: Bearer",
    `      credentials: ${token}`,
    "    tls_config:",
    "      insecure_skip_verify: true  # self-signed console certificate",
    "    static_configs:",
    `      - targets: ["${host}"]`,
  ].join("\n");
}

function CodeBlock({ text }: { text: string }) {
  const { t } = useI18n();
  return (
    <div className="relative">
      <pre className="overflow-x-auto rounded border border-line bg-surface-2 p-3 pr-12 font-mono text-xs leading-relaxed">{text}</pre>
      <Button className="absolute right-2 top-2" onClick={async () => { if (await copyText(text)) toast.success(t("copied")); }} aria-label={t("tokCopy")}><Copy className="h-3.5 w-3.5" /></Button>
    </div>
  );
}

export default function ApiTokens() {
  const { t, lang } = useI18n();
  const { isAdmin, role: myRole } = useCan();
  const list = useResource<ApiToken[]>(isAdmin ? "/api/tokens" : null, { poll: 30000 });
  const [q, setQ] = useState("");
  const [form, setForm] = useState<Form | null>(null);
  const [created, setCreated] = useState<Created | null>(null);
  const [deleting, setDeleting] = useState<ApiToken | null>(null);
  const { busy, error, setError, run } = useSubmit();
  const row = useSubmit();

  const rows = useMemo(() => {
    const s = q.trim().toLowerCase();
    return (list.data ?? []).filter((x) => !s || `${x.name} ${x.prefix}`.toLowerCase().includes(s));
  }, [list.data, q]);

  if (!isAdmin) {
    return (<><PageHeader title={t("navApiTokens")} description={t("tokensDesc")} />
      <Card><EmptyState icon={Lock} title={t("noPermissionTitle")} description={t("noPermissionDesc")} /></Card></>);
  }

  const open = () => { setError(null); setForm({ ...blank }); };
  const set = (p: Partial<Form>) => setForm((f) => (f ? { ...f, ...p } : f));
  const save = async () => {
    if (!form) return;
    const r = await run(() => api.post<Created>("/api/tokens", { name: form.name.trim(), role: form.role, expires_days: form.expires_days }), { deployed: false, success: t("tokCreatedToast") });
    if (r) { setForm(null); setCreated(r); list.reload(); }
  };
  const remove = async () => {
    if (!deleting) return;
    const r = await row.run(() => api.del(`/api/tokens/${deleting.id}`), { deployed: false, success: t("tokDeletedToast") });
    if (r !== undefined) list.reload();
  };
  const expired = (x: ApiToken) => !!x.expires_at && new Date(x.expires_at).getTime() <= Date.now();
  const addBtn = <Button variant="primary" onClick={open}><Plus className="h-4 w-4" />{t("addToken")}</Button>;

  return (
    <>
      <PageHeader title={t("navApiTokens")} description={t("tokensDesc")} actions={!list.missing && addBtn} />
      <Card>
        {list.missing ? <ComingSoon icon={KeySquare} /> : (<>
          <Toolbar><SearchInput className="w-full sm:w-72" value={q} onChange={setQ} placeholder={t("searchTokens")} /><span className="ml-auto text-xs text-fg-subtle num">{t("nItems", { n: rows.length })}</span></Toolbar>
          {list.loading ? <TableSkeleton cols={6} /> : list.error && !list.data ? <div className="p-4"><ErrorBox error={list.error} /></div> : !list.data?.length ? (
            <EmptyState icon={KeySquare} title={t("noTokens")} description={t("noTokensDesc")} action={addBtn} />
          ) : !rows.length ? <EmptyState icon={KeySquare} title={t("noResults")} description={t("noResultsDesc")} /> : (
            <Table>
              <THead><tr>
                <Th>{t("tokName")}</Th><Th className="hidden sm:table-cell">{t("tokPrefix")}</Th><Th>{t("role")}</Th>
                <Th className="hidden lg:table-cell">{t("tokCreatedBy")}</Th><Th className="hidden md:table-cell">{t("tokLastUsed")}</Th><Th>{t("tokExpires")}</Th>
                <Th className="w-12"><span className="sr-only">{t("actions")}</span></Th>
              </tr></THead>
              <tbody>
                {rows.map((x) => (
                  <Tr key={x.id}>
                    <Td><div className="font-medium">{x.name}</div><div className="font-mono text-xs text-fg-subtle sm:hidden">{x.prefix}…</div></Td>
                    <Td className="hidden font-mono text-xs text-fg-muted sm:table-cell">{x.prefix}…</Td>
                    <Td><Badge tone={roleTone[x.role] ?? "neutral"}>{roleKey[x.role] ? t(roleKey[x.role]) : x.role}</Badge></Td>
                    <Td className="hidden text-fg-muted lg:table-cell">{x.created_by_email || "–"}<div className="text-xs text-fg-subtle">{fmtDateTime(x.created_at, lang)}</div></Td>
                    <Td className="hidden text-fg-muted md:table-cell">{x.last_used_at ? relTime(x.last_used_at, lang) : <span className="text-xs text-fg-subtle">{t("tokNeverUsed")}</span>}</Td>
                    <Td>{expired(x) ? <Badge tone="danger" dot>{t("tokExpired")}</Badge> : x.expires_at ? <span className="text-fg-muted" title={fmtDateTime(x.expires_at, lang)}>{fmtDateTime(x.expires_at, lang)}</span> : <span className="text-xs text-fg-subtle">{t("tokNoExpiry")}</span>}</Td>
                    <Td className="text-right"><RowMenu items={[{ label: t("tokDelete"), icon: Trash2, danger: true, onSelect: () => setDeleting(x) }]} /></Td>
                  </Tr>
                ))}
              </tbody>
            </Table>
          )}
        </>)}
      </Card>

      <Callout className="mt-4" tone="info" icon={ShieldAlert}>{t("tokSessionNote")}</Callout>

      <Card className="mt-4">
        <CardHeader icon={Activity} title={t("tokPromTitle")} description={t("tokPromDesc")} />
        <div className="space-y-3 px-5 pb-5">
          <CodeBlock text={scrapeConfig("<token>")} />
          <p className="text-xs text-fg-subtle">{t("tokPromTls")}</p>
        </div>
      </Card>

      <Sheet open={!!form} onOpenChange={(o) => !o && setForm(null)} onSubmit={save} title={t("addToken")} description={t("tokSheetDesc")}
        footer={<><Button onClick={() => setForm(null)}>{t("cancel")}</Button><Button type="submit" variant="primary" loading={busy} disabled={!form?.name.trim()}>{t("tokCreate")}</Button></>}>
        {form && (<>
          {error && !fieldErr(error, "name") && !fieldErr(error, "role") && <div className="px-6 pt-5"><ErrorBox error={error} /></div>}
          <FormSection title={t("secBasic")}>
            <IdField label={t("tokName")} required hint={t("tokNameHint")} error={fieldErr(error, "name")}>
              {(id) => <Input id={id} autoComplete="off" maxLength={64} value={form.name} onChange={(e) => set({ name: e.target.value })} invalid={!!fieldErr(error, "name")} />}
            </IdField>
            <IdField label={t("role")} hint={t("tokRoleHint")} error={fieldErr(error, "role")}>
              {(id) => (
                <Select id={id} value={form.role} onChange={(e) => set({ role: e.target.value as Role })}>
                  {ROLES.filter((r) => RANK[r] <= (RANK[myRole] ?? 0)).map((r) => <option key={r} value={r}>{t(roleKey[r])}</option>)}
                </Select>
              )}
            </IdField>
            <IdField label={t("tokExpiry")} error={fieldErr(error, "expires_days")}>
              {(id) => (
                <Select id={id} value={String(form.expires_days)} onChange={(e) => set({ expires_days: Number(e.target.value) })}>
                  {EXPIRY.map((x) => <option key={x.days} value={x.days}>{t(x.key)}</option>)}
                </Select>
              )}
            </IdField>
          </FormSection>
        </>)}
      </Sheet>

      <Modal open={!!created} onOpenChange={(o) => !o && setCreated(null)} title={t("tokRevealTitle")} description={t("tokRevealDesc")} width="max-w-xl"
        footer={<Button variant="primary" onClick={() => setCreated(null)}>{t("tokSavedIt")}</Button>}>
        {created && (<div className="space-y-3">
          <Callout tone="warn" icon={ShieldAlert} title={t("tokRevealWarnTitle")}>{t("tokRevealWarn")}</Callout>
          <div className="flex gap-2">
            <Input readOnly aria-label={created.name} className="font-mono text-xs" value={created.token} onFocus={(e) => e.currentTarget.select()} />
            <Button className="shrink-0" onClick={async () => { if (await copyText(created.token)) toast.success(t("copied")); }}><Copy className="h-3.5 w-3.5" />{t("tokCopy")}</Button>
          </div>
          <div className="text-xs font-medium text-fg-muted">{t("tokPromTitle")}</div>
          <CodeBlock text={scrapeConfig(created.token)} />
        </div>)}
      </Modal>

      <ConfirmDialog open={!!deleting} onOpenChange={(o) => !o && setDeleting(null)} title={t("tokDeleteTitle")} description={t("tokDeleteDesc")} confirmLabel={t("tokDelete")} onConfirm={remove}>
        {deleting && <div className="rounded border border-line bg-surface-2 px-3 py-2 text-sm font-medium">{deleting.name} <span className="font-mono text-xs text-fg-subtle">· {deleting.prefix}…</span></div>}
      </ConfirmDialog>
    </>
  );
}
