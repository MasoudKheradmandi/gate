import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { toast } from "sonner";
import { AlertTriangle, Eye, ListChecks, Pencil, Plus, Trash2 } from "lucide-react";
import { ApiError, api, deployedVersion, type IpList } from "../api";
import { ErrorBox, PageHeader, Toolbar, useSubmit } from "../components/common";
import { Button } from "../components/ui/button";
import { Callout, Card, ComingSoon, EmptyState, TableSkeleton } from "../components/ui/display";
import { FormSection, IdField, Input, SearchInput, Textarea, fieldErr } from "../components/ui/form";
import { ConfirmDialog, RowMenu, Sheet } from "../components/ui/overlay";
import { Table, THead, Td, Th, Tr } from "../components/ui/table";
import { useResource } from "../hooks/useResource";
import { useCan } from "../lib/rbac";
import { useI18n } from "../i18n";
import { LIMITS, isIpOrCidr } from "../lib/rules";
import { cn, fmtDateTime, fmtNum, relTime } from "../lib/utils";

interface Form { name: string; description: string; text: string }
const blank: Form = { name: "", description: "", text: "" };
const parseItems = (text: string) => [...new Set(text.split(/\r?\n/).map((s) => s.trim()).filter(Boolean))];

export default function IpLists() {
  const { t, lang } = useI18n();
  const { can } = useCan();
  const list = useResource<IpList[]>("/api/ip-lists", { poll: 30000 });
  const [q, setQ] = useState("");
  const [editing, setEditing] = useState<{ id: number | null; form: Form; tried: boolean } | null>(null);
  const [deleting, setDeleting] = useState<IpList | null>(null);
  const { busy, error, setError, run } = useSubmit();
  const ro = !can;

  const filtered = useMemo(() => {
    const s = q.trim().toLowerCase();
    return (list.data ?? []).filter((r) => !s || `${r.name} ${r.description ?? ""}`.toLowerCase().includes(s) || r.items?.some((i) => i.includes(s)));
  }, [list.data, q]);
  const open = (r?: IpList) => { setError(null); setEditing({ id: r?.id ?? null, form: r ? { name: r.name, description: r.description ?? "", text: (r.items ?? []).join("\n") } : { ...blank }, tried: false }); };
  const done = () => { list.reload(); window.dispatchEvent(new Event("corvus:refresh")); };

  const f = editing?.form;
  const items = useMemo(() => (f ? parseItems(f.text) : []), [f]);
  const invalid = useMemo(() => items.filter((i) => !isIpOrCidr(i)), [items]);
  const clientErr = {
    name: f && !f.name.trim() ? t("fieldRequired") : undefined,
    items: invalid.length ? t("ipListInvalid", { n: invalid.length }) : items.length > LIMITS.listItems ? t("ipListTooMany", { n: fmtNum(LIMITS.listItems) }) : undefined,
  };
  const err = (k: "name" | "items" | "description") => (editing?.tried ? (clientErr as Record<string, string | undefined>)[k] : undefined) ?? fieldErr(error, k);

  const save = async () => {
    if (!editing || !f || ro) return;
    if (clientErr.name || clientErr.items) { setEditing((e) => (e ? { ...e, tried: true } : e)); return; }
    const body = { name: f.name.trim(), description: f.description.trim(), items };
    const r = await run(() => (editing.id ? api.put(`/api/ip-lists/${editing.id}`, body) : api.post("/api/ip-lists", body)), { success: t("savedDeployed") });
    if (r !== undefined) { setEditing(null); done(); }
  };
  const set = (p: Partial<Form>) => setEditing((e) => (e ? { ...e, form: { ...e.form, ...p } } : e));

  return (
    <>
      <PageHeader title={t("navIpLists")} description={t("ipListsDesc")}
        actions={can && !list.missing && <Button variant="primary" onClick={() => open()}><Plus className="h-4 w-4" />{t("addIpList")}</Button>} />
      <Card>
        {list.missing ? <ComingSoon icon={ListChecks} /> : (<>
          <Toolbar><SearchInput className="w-full sm:w-64" value={q} onChange={setQ} placeholder={t("searchIpLists")} /><span className="ml-auto text-xs text-fg-subtle num">{t("nItems", { n: filtered.length })}</span></Toolbar>
          {list.loading ? <TableSkeleton cols={4} /> : list.error && !list.data ? <div className="p-4"><ErrorBox error={list.error} /></div> : !list.data?.length ? (
            <EmptyState icon={ListChecks} title={t("noIpLists")} description={t("noIpListsDesc")} action={<Button variant="primary" onClick={() => open()}><Plus className="h-4 w-4" />{t("addIpList")}</Button>} />
          ) : !filtered.length ? <EmptyState icon={ListChecks} title={t("noResults")} description={t("noResultsDesc")} /> : (
            <Table>
              <THead><tr><Th>{t("name")}</Th><Th className="text-right">{t("entries")}</Th><Th className="hidden sm:table-cell">{t("usedByRules")}</Th><Th className="hidden md:table-cell">{t("updated")}</Th><Th className="w-12"><span className="sr-only">{t("actions")}</span></Th></tr></THead>
              <tbody>
                {filtered.map((r) => (
                  <Tr key={r.id}>
                    <Td className="max-w-[20rem]">
                      <button type="button" onClick={() => open(r)} className="block max-w-full truncate text-left font-medium text-fg hover:text-brand">{r.name}</button>
                      {r.description && <div className="truncate text-xs text-fg-subtle" title={r.description}>{r.description}</div>}
                    </Td>
                    <Td className="num text-right">{fmtNum(r.items?.length ?? 0)}</Td>
                    <Td className="hidden sm:table-cell">{r.rules ? <Link to="/security-rules" className="text-sm text-brand hover:underline">{t("rulesN", { n: r.rules })}</Link> : <span className="text-xs text-fg-subtle">{t("notUsed")}</span>}</Td>
                    <Td className="hidden text-fg-muted md:table-cell" title={r.updated_at ? fmtDateTime(r.updated_at, lang) : undefined}>{r.updated_at ? relTime(r.updated_at, lang) : "–"}</Td>
                    <Td className="text-right"><RowMenu items={[
                      { label: t("edit"), icon: Pencil, onSelect: () => open(r) },
                      { label: t("delete"), icon: Trash2, danger: true, separatorBefore: true, onSelect: () => setDeleting(r) },
                    ]} /></Td>
                  </Tr>
                ))}
              </tbody>
            </Table>
          )}
        </>)}
      </Card>

      <Sheet open={!!editing} onOpenChange={(o) => !o && setEditing(null)} onSubmit={save} title={ro ? t("viewIpList") : editing?.id ? t("editIpList") : t("addIpList")} description={t("ipListSheetDesc")}
        footer={ro ? <Button onClick={() => setEditing(null)}>{t("close")}</Button> : <><Button onClick={() => setEditing(null)}>{t("cancel")}</Button><Button type="submit" variant="primary" loading={busy}>{t("saveAndDeploy")}</Button></>}>
        {f && (<>
          {ro && <div className="px-6 pt-5"><Callout tone="neutral" icon={Eye}>{t("readOnlyNotice")}</Callout></div>}
          {error != null && !(error instanceof ApiError && error.kind === "validation" && error.fields.every((x) => x.field)) && <div className="px-6 pt-5"><ErrorBox error={error} /></div>}
          <FormSection title={t("secBasic")}>
            <IdField label={t("name")} required error={err("name")}>{(id) => <Input id={id} value={f.name} disabled={ro} maxLength={100} onChange={(e) => set({ name: e.target.value })} placeholder={t("ipListNamePh")} invalid={!!err("name")} />}</IdField>
            <IdField label={t("description")} error={err("description")}>{(id) => <Input id={id} value={f.description} disabled={ro} maxLength={500} onChange={(e) => set({ description: e.target.value })} />}</IdField>
          </FormSection>
          <FormSection title={t("entries")} description={t("ipListItemsDesc")}>
            <IdField label={t("ipListItems")} error={err("items")}
              hint={<span className={cn("num", items.length > LIMITS.listItems && "text-danger")}>{t("ipListCount", { n: fmtNum(items.length), max: fmtNum(LIMITS.listItems) })}</span>}>
              {(id) => <Textarea id={id} rows={14} spellCheck={false} readOnly={ro} className={cn("font-mono text-xs leading-5", ro && "bg-surface-2")} value={f.text}
                onChange={(e) => set({ text: e.target.value })} placeholder={"203.0.113.7\n10.0.0.0/8\n2001:db8::/32"} invalid={!!err("items")} />}
            </IdField>
            {!err("items") && <p className="num text-xs text-fg-subtle">{t("ipListCount", { n: fmtNum(items.length), max: fmtNum(LIMITS.listItems) })}</p>}
            {invalid.length > 0 && (
              <Callout tone="warn" icon={AlertTriangle} title={t("ipListInvalid", { n: invalid.length })}>
                <span className="font-mono text-xs">{invalid.slice(0, 5).join(", ")}{invalid.length > 5 ? " …" : ""}</span>
              </Callout>
            )}
          </FormSection>
        </>)}
      </Sheet>

      <DeleteIpList target={deleting} onClose={() => setDeleting(null)} onDone={done} />
    </>
  );
}

/** Delete confirmation that turns a 409 (still referenced by rules) into a friendly message. */
function DeleteIpList({ target, onClose, onDone }: { target: IpList | null; onClose: () => void; onDone: () => void }) {
  const { t } = useI18n();
  const run = async () => {
    if (!target) return;
    try {
      const r = await api.del(`/api/ip-lists/${target.id}`);
      const v = deployedVersion(r);
      toast.success(v != null ? `${t("deletedDeployed")} · ${t("versionN", { n: v })}` : t("deletedDeployed"));
      onDone();
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) toast.error(t("ipListInUse", { name: target.name }), { description: t("ipListInUseDesc") });
      else if (e instanceof ApiError && e.status === 403) toast.error(t("forbidden"));
      else toast.error(e instanceof ApiError ? e.message : t("requestFailed"));
    }
  };
  return (
    <ConfirmDialog open={!!target} onOpenChange={(o) => !o && onClose()} title={t("deleteIpListTitle")}
      description={target?.rules ? t("ipListInUseWarn", { n: target.rules }) : t("confirmDelete")} confirmLabel={t("delete")} onConfirm={run}>
      {target && <div className="rounded-lg border border-line bg-surface-2 px-3 py-2 text-sm font-medium">{target.name}</div>}
    </ConfirmDialog>
  );
}
