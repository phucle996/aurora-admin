import {
  BookOpenText,
  Database,
  FolderSync,
  KeyRound,
  Layers,
  ShieldCheck,
  TerminalSquare,
} from "lucide-react";
import { useTheme } from "next-themes";

import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

const quickStart = [
  "Login bằng Admin API key tại trang /login.",
  "Mở Runtime Module Status Board để xem module đã cài/chưa cài.",
  "Install module từ dialog và theo dõi log stream để debug.",
  "Xác nhận healthcheck + endpoint sau khi install thành công.",
];

const workflows = [
  {
    icon: Layers,
    title: "Module Lifecycle",
    detail: "Install, healthcheck, rollback schema khi thất bại và cập nhật endpoint.",
  },
  {
    icon: ShieldCheck,
    title: "TLS & mTLS",
    detail: "Admin ký cert service, copy cert/key/ca và ép traffic bảo mật giữa các service.",
  },
  {
    icon: KeyRound,
    title: "Token Secret Rotation",
    detail: "Rotate secret theo interval, đẩy cache invalidate để các service đồng bộ nhanh.",
  },
  {
    icon: Database,
    title: "Schema Runtime",
    detail: "Tạo schema riêng từng module, migrate trước khi SSH install binary/service.",
  },
  {
    icon: FolderSync,
    title: "Hosts Sync",
    detail: "Đồng bộ /etc/hosts giữa các node để gọi qua domain nội bộ thay vì IP thuần.",
  },
  {
    icon: TerminalSquare,
    title: "Debug Install",
    detail: "Dùng log stream để kiểm tra SSH, migration, healthcheck và cleanup rollback.",
  },
];

export default function AdminGuidePage() {
  const { resolvedTheme } = useTheme();
  const isDark = resolvedTheme !== "light";
  const panelClass = isDark ? "border-white/10 bg-slate-950/60" : "border-black/10 bg-white/85";
  const textPrimary = isDark ? "text-white" : "text-slate-900";
  const textMuted = isDark ? "text-slate-300" : "text-slate-600";

  return (
    <main className="space-y-4 py-3 lg:py-1">
      <header className="space-y-2">
        <Badge
          variant="outline"
          className={cn(
            "rounded-full px-3 py-1 text-xs uppercase tracking-[0.12em]",
            isDark ? "border-white/20 bg-white/5 text-slate-200" : "bg-white/70",
          )}
        >
          Admin Documentation
        </Badge>
        <h1 className={cn("text-3xl font-semibold tracking-tight", textPrimary)}>
          User Guide
        </h1>
        <p className={cn("text-sm", textMuted)}>
          Hướng dẫn thao tác nhanh cho vận hành, cài module và kiểm tra trạng thái runtime.
        </p>
      </header>

      <div className="grid gap-4 xl:grid-cols-[1.1fr_1fr]">
        <Card className={cn("shadow-lg backdrop-blur-xl", panelClass)}>
          <CardHeader>
            <CardTitle className={cn("flex items-center gap-2", textPrimary)}>
              <BookOpenText className="h-4 w-4 text-indigo-400" />
              Quick Start
            </CardTitle>
            <CardDescription className={textMuted}>
              Luồng thao tác khuyến nghị cho admin mới.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {quickStart.map((line, index) => (
              <div
                key={line}
                className={cn(
                  "rounded-lg border px-3 py-2 text-sm",
                  isDark ? "border-white/10 bg-white/5 text-slate-200" : "border-slate-200 bg-slate-50 text-slate-700",
                )}
              >
                <span className="mr-2 text-xs font-semibold text-indigo-400">B{index + 1}</span>
                {line}
              </div>
            ))}
          </CardContent>
        </Card>

        <Card className={cn("shadow-lg backdrop-blur-xl", panelClass)}>
          <CardHeader>
            <CardTitle className={textPrimary}>Operational Notes</CardTitle>
            <CardDescription className={textMuted}>
              Checklist ngắn để hạn chế lỗi cấu hình.
            </CardDescription>
          </CardHeader>
          <CardContent className={cn("space-y-2 text-sm", textMuted)}>
            <p>Luôn kiểm tra SSH host key fingerprint trước khi remote install.</p>
            <p>Với module cần DB, migration phải chạy trước bước cài service.</p>
            <p>Nếu install fail, cần xác nhận rollback schema và endpoint cleanup.</p>
            <p>Sau khi install, xác nhận endpoint healthcheck và hosts sync hoàn tất.</p>
          </CardContent>
        </Card>
      </div>

      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {workflows.map((item) => (
          <Card key={item.title} className={cn("shadow-lg backdrop-blur-xl", panelClass)}>
            <CardHeader className="pb-2">
              <CardTitle className={cn("flex items-center gap-2 text-base", textPrimary)}>
                <item.icon className="h-4 w-4 text-indigo-400" />
                {item.title}
              </CardTitle>
            </CardHeader>
            <CardContent className={cn("text-sm", textMuted)}>{item.detail}</CardContent>
          </Card>
        ))}
      </section>
    </main>
  );
}
