import {
  Activity,
  AlertTriangle,
  BellRing,
  CheckCircle2,
  Clock3,
  Fingerprint,
  KeyRound,
  LifeBuoy,
  NotebookTabs,
  Server,
  ShieldCheck,
  Users,
} from "lucide-react";
import { useTheme } from "next-themes";
import { Link, useLocation } from "react-router-dom";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

const metrics = [
  { label: "Active Admins", value: "12", delta: "+2 today" },
  { label: "Security Alerts", value: "03", delta: "-1 vs yesterday" },
  { label: "Pending Reviews", value: "17", delta: "+5 this week" },
  { label: "API Requests", value: "48,291", delta: "+8.2%" },
];

const activities = [
  { icon: ShieldCheck, text: "Admin key validated for root console", time: "2m ago" },
  { icon: Users, text: "New administrator invited", time: "15m ago" },
  { icon: KeyRound, text: "Admin key rotation scheduled", time: "1h ago" },
  { icon: Clock3, text: "Daily backup completed", time: "3h ago" },
];

const services = [
  { name: "Auth Gateway", status: "Healthy" },
  { name: "Audit Log Service", status: "Healthy" },
  { name: "Notification Queue", status: "Degraded" },
  { name: "Storage Sync", status: "Healthy" },
];

const rightActions = [
  { label: "Open Login", description: "Kiểm tra lại admin key", to: "/login", icon: KeyRound },
  {
    label: "404 Preview",
    description: "Xem nhanh trang not found",
    to: "/not-found-preview",
    icon: NotebookTabs,
  },
];

const moduleLabels: Record<string, string> = {
  vm: "VM Fleet",
  docker: "Docker",
  k8s: "Kubernetes",
  revenue: "Doanh thu",
  cost: "Chi phi",
};

export default function DashboardPage() {
  const { resolvedTheme } = useTheme();
  const location = useLocation();

  const isDark = resolvedTheme !== "light";
  const activeModule = new URLSearchParams(location.search).get("module") ?? "vm";
  const activeModuleLabel = moduleLabels[activeModule] ?? "VM Fleet";
  const textPrimary = isDark ? "text-white" : "text-slate-900";
  const textMuted = isDark ? "text-slate-300" : "text-slate-600";
  const panelClass = isDark
    ? "border-white/10 bg-slate-950/60"
    : "border-black/10 bg-white/85";

  return (
    <main className="space-y-4 py-3 lg:py-1">
      <header className="mb-5 flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-2">
          <Badge
            variant="outline"
            className={cn(
              "rounded-full px-3 py-1 text-xs uppercase tracking-[0.12em]",
              isDark ? "border-white/20 bg-white/5 text-slate-200" : "bg-white/70",
            )}
          >
            Admin Dashboard
          </Badge>
          <h1 className={cn("text-3xl font-semibold tracking-tight", textPrimary)}>
            {activeModuleLabel} Overview
          </h1>
          <p className={cn("text-sm", textMuted)}>
            Tổng quan bảo mật, hoạt động vận hành và trạng thái dịch vụ theo module đang chọn.
          </p>
        </div>
      </header>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_22rem]">
        <div className="space-y-4">
          <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            {metrics.map((metric) => (
              <Card key={metric.label} className={cn("shadow-lg backdrop-blur-xl", panelClass)}>
                <CardHeader className="pb-2">
                  <CardDescription className={textMuted}>{metric.label}</CardDescription>
                  <CardTitle className={cn("text-3xl", textPrimary)}>{metric.value}</CardTitle>
                </CardHeader>
                <CardContent className="text-xs text-indigo-300 dark:text-indigo-200">
                  {metric.delta}
                </CardContent>
              </Card>
            ))}
          </section>

          <section className="grid gap-4 lg:grid-cols-[1.3fr_1fr]">
            <Card className={cn("shadow-lg backdrop-blur-xl", panelClass)}>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <div>
                    <CardTitle className={textPrimary}>Recent Security Activity</CardTitle>
                    <CardDescription className={textMuted}>
                      Sự kiện xác thực và thay đổi quyền truy cập gần nhất.
                    </CardDescription>
                  </div>
                  <Activity className={cn("h-4 w-4", isDark ? "text-slate-300" : "text-slate-500")} />
                </div>
              </CardHeader>
              <CardContent className="space-y-3">
                {activities.map((item, index) => (
                  <div
                    key={index}
                    className={cn(
                      "flex items-center justify-between rounded-lg border px-3 py-2",
                      isDark ? "border-white/10 bg-white/5" : "border-slate-200/80 bg-slate-50/80",
                    )}
                  >
                    <div className={cn("flex items-center gap-2 text-sm", textPrimary)}>
                      <item.icon className="h-4 w-4 text-indigo-400" />
                      {item.text}
                    </div>
                    <span className={cn("text-xs", textMuted)}>{item.time}</span>
                  </div>
                ))}
              </CardContent>
            </Card>

            <Card className={cn("shadow-lg backdrop-blur-xl", panelClass)}>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <div>
                    <CardTitle className={textPrimary}>Service Status</CardTitle>
                    <CardDescription className={textMuted}>
                      Tình trạng các thành phần trọng yếu của hệ thống.
                    </CardDescription>
                  </div>
                  <Server className={cn("h-4 w-4", isDark ? "text-slate-300" : "text-slate-500")} />
                </div>
              </CardHeader>
              <CardContent className="space-y-3">
                {services.map((service) => {
                  const healthy = service.status === "Healthy";
                  return (
                    <div
                      key={service.name}
                      className={cn(
                        "flex items-center justify-between rounded-lg border px-3 py-2",
                        isDark ? "border-white/10 bg-white/5" : "border-slate-200/80 bg-slate-50/80",
                      )}
                    >
                      <span className={cn("text-sm", textPrimary)}>{service.name}</span>
                      <span className="inline-flex items-center gap-1 text-xs">
                        {healthy ? (
                          <CheckCircle2 className="h-3.5 w-3.5 text-emerald-500" />
                        ) : (
                          <AlertTriangle className="h-3.5 w-3.5 text-amber-500" />
                        )}
                        <span className={healthy ? "text-emerald-500" : "text-amber-500"}>
                          {service.status}
                        </span>
                      </span>
                    </div>
                  );
                })}
              </CardContent>
            </Card>
          </section>
        </div>

        <aside className="space-y-4 xl:sticky xl:top-8 xl:h-fit">
          <Card className={cn("shadow-lg backdrop-blur-xl", panelClass)}>
            <CardHeader>
              <CardTitle className={cn("flex items-center gap-2", textPrimary)}>
                <Fingerprint className="h-4 w-4 text-indigo-400" />
                Right Sidebar
              </CardTitle>
              <CardDescription className={textMuted}>Quick admin actions</CardDescription>
            </CardHeader>
            <CardContent className="space-y-2">
              {rightActions.map((action) => (
                <Button
                  key={action.label}
                  asChild
                  variant="outline"
                  className={cn(
                    "h-auto w-full justify-start gap-2 px-3 py-2 text-left",
                    isDark ? "border-white/15 bg-white/5 hover:bg-white/10" : "bg-white",
                  )}
                >
                  <Link to={action.to}>
                    <action.icon className="h-4 w-4 text-indigo-400" />
                    <span className="flex flex-col">
                      <span className={cn("text-sm font-medium", textPrimary)}>{action.label}</span>
                      <span className={cn("text-xs", textMuted)}>{action.description}</span>
                    </span>
                  </Link>
                </Button>
              ))}
            </CardContent>
          </Card>

          <Card className={cn("shadow-lg backdrop-blur-xl", panelClass)}>
            <CardHeader>
              <CardTitle className={cn("flex items-center gap-2", textPrimary)}>
                <BellRing className="h-4 w-4 text-indigo-400" />
                Session Notes
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div
                className={cn(
                  "rounded-lg border px-3 py-2 text-xs",
                  isDark ? "border-white/10 bg-white/5 text-slate-200" : "border-slate-200 bg-slate-50",
                )}
              >
                Admin key session is active.
              </div>
              <div
                className={cn(
                  "rounded-lg border px-3 py-2 text-xs",
                  isDark
                    ? "border-white/10 bg-white/5 text-slate-300"
                    : "border-slate-200 bg-slate-50 text-slate-600",
                )}
              >
                Last refresh: {new Date().toLocaleTimeString("vi-VN")}
              </div>
            </CardContent>
          </Card>

          <Card className={cn("shadow-lg backdrop-blur-xl", panelClass)}>
            <CardHeader>
              <CardTitle className={cn("flex items-center gap-2", textPrimary)}>
                <LifeBuoy className="h-4 w-4 text-indigo-400" />
                Support
              </CardTitle>
              <CardDescription className={textMuted}>
                Nếu cần hỗ trợ khẩn, kiểm tra log và liên hệ đội vận hành.
              </CardDescription>
            </CardHeader>
          </Card>
        </aside>
      </div>
    </main>
  );
}
