import { useState } from "react";
import { ArrowRight, KeyRound, ShieldCheck } from "lucide-react";
import { useTheme } from "next-themes";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";

import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { AuthLayout } from "./Authlayout";
import { leftBackgrounds } from "./auth-layout-theme";
import {
  getAdminAuthErrorMessage,
  loginWithAPIKey,
  setAdminSession,
} from "@/lib/admin-auth";
import { useEnabledModules } from "@/state/enabled-modules-context";

export default function LoginPage() {
  const navigate = useNavigate();
  const { refreshModules, clearModules } = useEnabledModules();
  const [adminKey, setAdminKey] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const { resolvedTheme } = useTheme();

  const isDark = resolvedTheme !== "light";
  const canSubmit = adminKey.trim().length > 0;

  const textPrimary = isDark ? "text-white" : "text-slate-900";
  const textMuted = isDark ? "text-slate-300" : "text-slate-600";
  const labelColor = isDark ? "text-slate-200" : "text-slate-700";
  const panelClass = isDark
    ? "border-white/10 bg-black/20 shadow-black/30"
    : "border-black/10 bg-white/75 shadow-slate-300/60";

  const hero = (
    <div className="space-y-5">
      <div
        className={cn(
          "inline-flex items-center gap-2 rounded-full border px-3 py-1 text-xs font-semibold uppercase tracking-[0.12em]",
          isDark
            ? "border-white/20 bg-white/5 text-white"
            : "border-black/15 bg-white/40 text-slate-800",
        )}
      >
        <ShieldCheck className="h-3.5 w-3.5" />
        Admin Console
      </div>
      <div
        className={cn("space-y-2", isDark ? "text-white" : "text-slate-900")}
      >
        <h2 className="text-4xl font-semibold leading-tight">Secure Access</h2>
        <p className={cn("max-w-sm text-sm leading-relaxed", textMuted)}>
          Dùng admin key để truy cập khu vực quản trị. Mọi thao tác sẽ được kiểm
          soát theo chính sách bảo mật nội bộ.
        </p>
      </div>
    </div>
  );

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canSubmit || submitting) return;

    setSubmitting(true);

    try {
      await loginWithAPIKey(adminKey);
      setAdminSession(true);
      try {
        await refreshModules({ force: true });
      } catch {
        toast.warning("Không thể đồng bộ module list, đang dùng cache hiện có");
      }
      toast.success("Xác thực thành công");
      navigate("/dashboard");
    } catch (error) {
      setAdminSession(false);
      clearModules();
      toast.error(getAdminAuthErrorMessage(error));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="min-h-screen w-full">
      <div className="grid min-h-screen w-full overflow-hidden lg:grid-cols-[1.05fr_1fr]">
        <section className="relative">
          <div
            className="absolute inset-0 transition-[background] duration-500 ease-out"
            style={{
              background: isDark ? leftBackgrounds.dark : leftBackgrounds.light,
            }}
          />
          <div className="relative z-10 flex min-h-screen items-center justify-center px-6 py-12 md:px-10 lg:px-16">
            <div
              className={cn(
                "w-full max-w-lg rounded-3xl border p-7 shadow-2xl backdrop-blur-xl md:p-9",
                panelClass,
              )}
            >
              <header className="space-y-2">
                <p
                  className={cn(
                    "text-xs font-semibold uppercase tracking-[0.14em]",
                    textMuted,
                  )}
                >
                  Admin Authentication
                </p>
                <h1 className={cn("text-3xl font-semibold", textPrimary)}>
                  Đăng nhập quản trị
                </h1>
                <p className={cn("text-sm leading-relaxed", textMuted)}>
                  Nhập admin key để mở phiên làm việc dành cho quản trị viên.
                </p>
              </header>

              <form className="mt-8 space-y-6" onSubmit={handleSubmit}>
                <FieldGroup className="gap-5">
                  <Field className="gap-2">
                    <FieldLabel htmlFor="admin-key" className={labelColor}>
                      Admin key
                    </FieldLabel>
                    <div className="relative">
                      <KeyRound
                        className={cn(
                          "pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2",
                          isDark ? "text-slate-400" : "text-slate-500",
                        )}
                      />
                      <Input
                        id="admin-key"
                        type="password"
                        autoComplete="current-password"
                        value={adminKey}
                        onChange={(event) => setAdminKey(event.target.value)}
                        placeholder="Nhập admin key"
                        className={cn(
                          "h-12 pl-10 text-base",
                          isDark
                            ? "border-white/15 bg-black/20 text-white placeholder:text-slate-400"
                            : "bg-white/85",
                        )}
                        required
                      />
                    </div>
                    <FieldDescription className={textMuted}>
                      Dùng API key quản trị để đăng nhập.
                    </FieldDescription>
                  </Field>
                </FieldGroup>

                <Button
                  type="submit"
                  size="lg"
                  disabled={!canSubmit || submitting}
                  className="h-12 w-full bg-indigo-500 text-white hover:bg-indigo-400"
                >
                  {submitting ? "Đang xác thực..." : "Đăng nhập bằng admin key"}
                  {!submitting && <ArrowRight className="h-4 w-4" />}
                </Button>
              </form>
            </div>
          </div>
        </section>

        <AuthLayout hero={hero} />
      </div>
    </div>
  );
}
