import { useState } from "react";
import { Check, Copy, Download, KeyRound, Lock, ShieldCheck, ShieldOff, Smartphone } from "lucide-react";
import { toast } from "sonner";
import { api, isMissing, type MfaSetup, type User } from "../api";
import { ErrorBox, PageHeader, useSubmit } from "../components/common";
import { Button } from "../components/ui/button";
import { Badge, Callout, Card, CardHeader } from "../components/ui/display";
import { IdField, Input, fieldErr } from "../components/ui/form";
import { Modal, Sheet } from "../components/ui/overlay";
import { useApp } from "../lib/app";
import { useI18n } from "../i18n";
import { roleKey } from "../lib/rbac";
import { copyText, download, fmtDateTime } from "../lib/utils";
import { CodeInput } from "./Auth";

export default function Account() {
  const { t, lang } = useI18n();
  const { user, setUser } = useApp();

  const refreshUser = async () => { try { setUser(await api.get<User>("/api/auth/me")); } catch { /* ignore */ } };

  return (
    <>
      <PageHeader title={t("navAccount")} description={t("accountDesc")} />
      <div className="grid max-w-5xl items-start gap-5 lg:grid-cols-2">
        <Card className="lg:col-span-2">
          <div className="flex flex-wrap items-center gap-4 p-4">
            <span className="grid h-12 w-12 place-items-center rounded-full bg-brand-solid text-lg font-semibold text-on-brand">{(user.name || user.email).charAt(0).toUpperCase()}</span>
            <div className="min-w-0 flex-1"><div className="font-semibold">{user.name || user.email}</div><div className="text-sm text-fg-subtle">{user.email}</div></div>
            <dl className="flex gap-6 text-xs">
              <div><dt className="text-fg-subtle">{t("role")}</dt><dd className="mt-0.5 font-medium">{roleKey[user.role] ? t(roleKey[user.role]) : user.role}</dd></div>
              <div><dt className="text-fg-subtle">{t("lastLogin")}</dt><dd className="mt-0.5 font-medium num">{fmtDateTime(user.last_login_at, lang)}</dd></div>
            </dl>
          </div>
        </Card>
        <PasswordCard />
        <MfaCard user={user} onChanged={refreshUser} />
      </div>
    </>
  );
}

function PasswordCard() {
  const { t } = useI18n();
  const [f, setF] = useState({ current: "", new: "", confirm: "" });
  const { busy, error, run } = useSubmit();
  const mismatch = f.confirm.length > 0 && f.new !== f.confirm;
  const submit = async () => {
    if (mismatch) return;
    const r = await run(() => api.post("/api/auth/password", { current: f.current, new: f.new }), { success: t("passwordChanged"), deployed: false });
    if (r !== undefined) setF({ current: "", new: "", confirm: "" });
  };
  return (
    <Card>
      <CardHeader icon={Lock} title={t("changePassword")} description={t("changePasswordDesc")} />
      <form className="space-y-4 px-4 pb-4 pt-2" onSubmit={(e) => { e.preventDefault(); submit(); }}>
        {!fieldErr(error, "new", "current") && <ErrorBox error={error} />}
        <IdField label={t("currentPassword")} error={fieldErr(error, "current")}>{(id) => <Input id={id} type="password" autoComplete="current-password" required value={f.current} onChange={(e) => setF({ ...f, current: e.target.value })} invalid={!!fieldErr(error, "current")} />}</IdField>
        <IdField label={t("newPassword")} hint={t("passwordHint")} error={fieldErr(error, "new")}>{(id) => <Input id={id} type="password" autoComplete="new-password" required minLength={10} value={f.new} onChange={(e) => setF({ ...f, new: e.target.value })} invalid={!!fieldErr(error, "new")} />}</IdField>
        <IdField label={t("confirmPassword")} error={mismatch ? t("passwordMismatch") : undefined}>{(id) => <Input id={id} type="password" autoComplete="new-password" required value={f.confirm} onChange={(e) => setF({ ...f, confirm: e.target.value })} invalid={mismatch} />}</IdField>
        <p className="text-xs text-fg-subtle">{t("passwordChangeNote")}</p>
        <div className="flex justify-end"><Button type="submit" variant="primary" loading={busy} disabled={!f.current || !f.new || mismatch}>{t("changePassword")}</Button></div>
      </form>
    </Card>
  );
}

function MfaCard({ user, onChanged }: { user: User; onChanged: () => void }) {
  const { t } = useI18n();
  const [wizard, setWizard] = useState<{ setup: MfaSetup; code: string; codes: string[] | null } | null>(null);
  const [disabling, setDisabling] = useState(false);
  const start = useSubmit();
  const confirm = useSubmit();
  const off = useSubmit();
  const [dis, setDis] = useState({ password: "", code: "" });
  const missing = isMissing(start.error);

  const begin = async () => {
    const s = await start.run(() => api.post<MfaSetup>("/api/auth/mfa/setup"), { deployed: false, silentError: true });
    if (s) { confirm.setError(null); setWizard({ setup: s, code: "", codes: null }); }
  };
  const enable = async (code: string) => {
    const r = await confirm.run(() => api.post<{ recovery_codes: string[] }>("/api/auth/mfa/enable", { code }), { deployed: false, silentError: true });
    if (r) { setWizard((w) => (w ? { ...w, codes: r.recovery_codes ?? [] } : w)); onChanged(); }
  };
  const disable = async () => {
    const r = await off.run(() => api.post("/api/auth/mfa/disable", dis), { success: t("mfaDisabled"), deployed: false });
    if (r !== undefined) { setDisabling(false); setDis({ password: "", code: "" }); onChanged(); }
  };
  const enabled = !!user.mfa_enabled;

  return (
    <Card>
      <CardHeader icon={ShieldCheck} title={t("mfaCardTitle")} description={t("mfaCardDesc")} actions={enabled ? <Badge tone="ok" dot>{t("on")}</Badge> : <Badge>{t("off")}</Badge>} />
      <div className="space-y-4 px-4 pb-4 pt-2">
        {missing ? <Callout tone="neutral" icon={Smartphone} title={t("comingSoon")}>{t("comingSoonDesc")}</Callout> : <>
          {!missing && start.error ? <ErrorBox error={start.error} /> : null}
          <div className="flex items-start gap-3 rounded-lg border border-line bg-surface-2/50 p-3.5">
            <span className={`grid h-9 w-9 shrink-0 place-items-center rounded-lg ${enabled ? "bg-ok/15 text-ok" : "bg-surface-3 text-fg-subtle"}`}>{enabled ? <ShieldCheck className="h-4 w-4" /> : <ShieldOff className="h-4 w-4" />}</span>
            <div className="min-w-0 text-sm"><div className="font-medium">{enabled ? t("mfaOnTitle") : t("mfaOffTitle")}</div><div className="mt-0.5 text-xs leading-5 text-fg-subtle">{enabled ? t("mfaOnDesc") : t("mfaOffDesc")}</div></div>
          </div>
          <div className="flex justify-end">
            {enabled ? <Button variant="danger-ghost" onClick={() => { off.setError(null); setDisabling(true); }}>{t("mfaDisable")}</Button>
              : <Button variant="primary" loading={start.busy} onClick={begin}><Smartphone className="h-4 w-4" />{t("mfaEnable")}</Button>}
          </div>
        </>}
      </div>

      <Sheet open={!!wizard} onOpenChange={(o) => { if (!o) setWizard(null); }} title={wizard?.codes ? t("mfaRecoveryTitle") : t("mfaSetupTitle")} description={wizard?.codes ? t("mfaRecoveryDesc") : t("mfaSetupDesc")}
        footer={wizard?.codes ? <Button variant="primary" onClick={() => setWizard(null)}><Check className="h-4 w-4" />{t("mfaSavedCodes")}</Button> : <Button onClick={() => setWizard(null)}>{t("cancel")}</Button>}>
        {wizard && !wizard.codes && (
          <div className="space-y-6 px-6 py-5">
            <ol className="space-y-5 text-sm">
              <li className="flex gap-3"><Step n={1} /><div className="flex-1"><div className="font-medium">{t("mfaStep1")}</div>
                <div className="mt-3 flex flex-col items-center gap-3 rounded-xl border border-line bg-white p-4 sm:flex-row sm:items-center">
                  <img src={wizard.setup.qr_png} alt="MFA QR code" width={168} height={168} className="h-[168px] w-[168px] [image-rendering:pixelated]" />
                  <div className="min-w-0 flex-1 text-xs text-neutral-600"><div className="mb-1 font-medium text-neutral-800">{t("mfaManualKey")}</div>
                    <code className="block break-all rounded bg-neutral-100 px-2 py-1.5 font-mono text-[11px] text-neutral-800">{wizard.setup.secret}</code>
                    <button type="button" onClick={async () => { if (await copyText(wizard.setup.secret)) toast.success(t("copied")); }} className="mt-2 inline-flex items-center gap-1 text-xs font-medium text-indigo-600 hover:underline"><Copy className="h-3 w-3" />{t("copy")}</button></div>
                </div></div></li>
              <li className="flex gap-3"><Step n={2} /><div className="flex-1"><div className="font-medium">{t("mfaStep2")}</div>
                <div className="mt-3 space-y-3">
                  <ErrorBox error={confirm.error} />
                  <CodeInput value={wizard.code} onChange={(c) => setWizard({ ...wizard, code: c })} onComplete={enable} disabled={confirm.busy} />
                  <Button variant="primary" className="w-full justify-center" loading={confirm.busy} disabled={wizard.code.length !== 6} onClick={() => enable(wizard.code)}>{t("verifyAndEnable")}</Button>
                </div></div></li>
            </ol>
          </div>
        )}
        {wizard?.codes && (
          <div className="space-y-4 px-6 py-5">
            <Callout tone="warn" icon={KeyRound} title={t("mfaRecoveryWarnTitle")}>{t("mfaRecoveryWarn")}</Callout>
            <div className="grid grid-cols-2 gap-2">
              {wizard.codes.map((c) => <code key={c} className="rounded-lg border border-line bg-surface-2 px-3 py-2 text-center font-mono text-sm tracking-wider">{c}</code>)}
            </div>
            <div className="flex gap-2">
              <Button onClick={async () => { if (await copyText(wizard.codes!.join("\n"))) toast.success(t("copied")); }}><Copy className="h-3.5 w-3.5" />{t("copyAll")}</Button>
              <Button onClick={() => download("corvus-recovery-codes.txt", wizard.codes!.join("\n") + "\n")}><Download className="h-3.5 w-3.5" />{t("download")}</Button>
            </div>
          </div>
        )}
      </Sheet>

      <Modal open={disabling} onOpenChange={setDisabling} title={t("mfaDisable")} description={t("mfaDisableDesc")}
        footer={<><Button onClick={() => setDisabling(false)}>{t("cancel")}</Button><Button variant="danger" loading={off.busy} disabled={!dis.password || dis.code.length < 6} onClick={disable}>{t("mfaDisable")}</Button></>}>
        <div className="space-y-4">
          <ErrorBox error={off.error} />
          <IdField label={t("password")} error={fieldErr(off.error, "password")}>{(id) => <Input id={id} type="password" autoComplete="current-password" value={dis.password} onChange={(e) => setDis({ ...dis, password: e.target.value })} />}</IdField>
          <IdField label={t("mfaCodeOrRecovery")} error={fieldErr(off.error, "code")}>{(id) => <Input id={id} inputMode="numeric" autoComplete="one-time-code" className="font-mono" value={dis.code} onChange={(e) => setDis({ ...dis, code: e.target.value })} />}</IdField>
        </div>
      </Modal>
    </Card>
  );
}

const Step = ({ n }: { n: number }) => <span className="mt-0.5 grid h-5 w-5 shrink-0 place-items-center rounded-full bg-brand/15 text-2xs font-semibold text-brand">{n}</span>;
