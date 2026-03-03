import { useState } from "react";
import { Bell, KeyRound, Save, Send, Settings2 } from "lucide-react";
import { useTheme } from "next-themes";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";

type DeliveryChannel = "off" | "telegram" | "slack" | "email";

export default function AdminSettingsPage() {
  const { resolvedTheme } = useTheme();
  const isDark = resolvedTheme !== "light";

  const panelClass = isDark ? "border-white/10 bg-slate-950/60" : "border-black/10 bg-white/85";
  const textPrimary = isDark ? "text-white" : "text-slate-900";
  const textMuted = isDark ? "text-slate-300" : "text-slate-600";

  const [telegramEnabled, setTelegramEnabled] = useState(true);
  const [telegramBotToken, setTelegramBotToken] = useState("");
  const [telegramChatID, setTelegramChatID] = useState("");

  const [slackEnabled, setSlackEnabled] = useState(false);
  const [slackWebhookURL, setSlackWebhookURL] = useState("");

  const [emailEnabled, setEmailEnabled] = useState(false);
  const [emailRecipients, setEmailRecipients] = useState("");

  const [sendAdminKeyEnabled, setSendAdminKeyEnabled] = useState(false);
  const [sendAdminKeyChannel, setSendAdminKeyChannel] = useState<DeliveryChannel>("off");

  const handleToggleSendAdminKey = (checked: boolean) => {
    setSendAdminKeyEnabled(checked);
    if (!checked) {
      setSendAdminKeyChannel("off");
    } else if (sendAdminKeyChannel === "off") {
      setSendAdminKeyChannel("telegram");
    }
  };

  const handleSave = () => {
    if (sendAdminKeyEnabled && sendAdminKeyChannel === "off") {
      toast.error("Send Admin Key đang bật, cần chọn 1 kênh gửi.");
      return;
    }

    if (sendAdminKeyEnabled) {
      const channelEnabled =
        (sendAdminKeyChannel === "telegram" && telegramEnabled) ||
        (sendAdminKeyChannel === "slack" && slackEnabled) ||
        (sendAdminKeyChannel === "email" && emailEnabled);

      if (!channelEnabled) {
        toast.error("Kênh đã chọn cho Send Admin Key đang tắt.");
        return;
      }
    }

    toast.success("Đã lưu cấu hình notification");
  };

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
          Admin Settings
        </Badge>
        <h1 className={cn("text-3xl font-semibold tracking-tight", textPrimary)}>
          Notification & Secret Delivery
        </h1>
        <p className={cn("text-sm", textMuted)}>
          Quản lý kênh thông báo và rule gửi Admin API Key.
        </p>
      </header>

      <div className="grid gap-4 xl:grid-cols-[1.2fr_1fr]">
        <section className="space-y-4">
          <Card className={cn("shadow-lg backdrop-blur-xl", panelClass)}>
            <CardHeader>
              <CardTitle className={cn("flex items-center gap-2", textPrimary)}>
                <Bell className="h-4 w-4 text-indigo-400" />
                Notification Channels
              </CardTitle>
              <CardDescription className={textMuted}>
                Bật/tắt và cấu hình từng kênh gửi thông báo.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="rounded-xl border p-3">
                <div className="mb-3 flex items-center justify-between">
                  <Label className={textPrimary}>Telegram</Label>
                  <Switch checked={telegramEnabled} onCheckedChange={setTelegramEnabled} />
                </div>
                <div className="grid gap-3 md:grid-cols-2">
                  <Input
                    placeholder="Bot token"
                    value={telegramBotToken}
                    onChange={(e) => setTelegramBotToken(e.target.value)}
                    disabled={!telegramEnabled}
                  />
                  <Input
                    placeholder="Chat ID"
                    value={telegramChatID}
                    onChange={(e) => setTelegramChatID(e.target.value)}
                    disabled={!telegramEnabled}
                  />
                </div>
              </div>

              <div className="rounded-xl border p-3">
                <div className="mb-3 flex items-center justify-between">
                  <Label className={textPrimary}>Slack</Label>
                  <Switch checked={slackEnabled} onCheckedChange={setSlackEnabled} />
                </div>
                <Input
                  placeholder="Webhook URL"
                  value={slackWebhookURL}
                  onChange={(e) => setSlackWebhookURL(e.target.value)}
                  disabled={!slackEnabled}
                />
              </div>

              <div className="rounded-xl border p-3">
                <div className="mb-3 flex items-center justify-between">
                  <Label className={textPrimary}>Email</Label>
                  <Switch checked={emailEnabled} onCheckedChange={setEmailEnabled} />
                </div>
                <Textarea
                  placeholder="ops@example.com, sec@example.com"
                  value={emailRecipients}
                  onChange={(e) => setEmailRecipients(e.target.value)}
                  disabled={!emailEnabled}
                />
              </div>
            </CardContent>
          </Card>
        </section>

        <aside className="space-y-4 xl:sticky xl:top-8 xl:h-fit">
          <Card className={cn("shadow-lg backdrop-blur-xl", panelClass)}>
            <CardHeader>
              <CardTitle className={cn("flex items-center gap-2", textPrimary)}>
                <KeyRound className="h-4 w-4 text-indigo-400" />
                Send Admin Key
              </CardTitle>
              <CardDescription className={textMuted}>
                Cho phép gửi Admin API key qua đúng 1 kênh hoặc tắt hoàn toàn.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center justify-between rounded-xl border p-3">
                <div>
                  <p className={cn("text-sm font-medium", textPrimary)}>Enable send admin key</p>
                  <p className={cn("text-xs", textMuted)}>Nếu tắt, không channel nào được dùng.</p>
                </div>
                <Switch checked={sendAdminKeyEnabled} onCheckedChange={handleToggleSendAdminKey} />
              </div>

              <RadioGroup
                value={sendAdminKeyEnabled ? sendAdminKeyChannel : "off"}
                onValueChange={(value) => setSendAdminKeyChannel(value as DeliveryChannel)}
                className="rounded-xl border p-3"
              >
                <Label className={cn("mb-2 block text-xs uppercase tracking-[0.1em]", textMuted)}>
                  Delivery Channel
                </Label>

                {[
                  { value: "off", label: "OFF", desc: "Không gửi admin key." },
                  { value: "telegram", label: "Telegram", desc: "Gửi qua bot Telegram." },
                  { value: "slack", label: "Slack", desc: "Gửi vào Slack webhook." },
                  { value: "email", label: "Email", desc: "Gửi tới email recipients." },
                ].map((item) => (
                  <label
                    key={item.value}
                    className={cn(
                      "mb-2 flex cursor-pointer items-start gap-2 rounded-lg border px-2 py-2",
                      isDark ? "border-white/10 hover:bg-white/5" : "hover:bg-slate-50",
                    )}
                  >
                    <RadioGroupItem
                      value={item.value}
                      disabled={!sendAdminKeyEnabled && item.value !== "off"}
                    />
                    <div>
                      <p className={cn("text-sm font-medium", textPrimary)}>{item.label}</p>
                      <p className={cn("text-xs", textMuted)}>{item.desc}</p>
                    </div>
                  </label>
                ))}
              </RadioGroup>

              <p className={cn("text-xs", textMuted)}>
                Rule hiện tại: chỉ 1 channel được chọn cho Send Admin Key.
              </p>
            </CardContent>
          </Card>

          <Card className={cn("shadow-lg backdrop-blur-xl", panelClass)}>
            <CardHeader>
              <CardTitle className={cn("flex items-center gap-2", textPrimary)}>
                <Settings2 className="h-4 w-4 text-indigo-400" />
                Actions
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              <Button className="w-full gap-2" onClick={handleSave}>
                <Save className="h-4 w-4" />
                Save Settings
              </Button>
              <Button
                variant="outline"
                className="w-full gap-2"
                onClick={() => toast.info("Trigger test notification")}
              >
                <Send className="h-4 w-4" />
                Send Test Notification
              </Button>
            </CardContent>
          </Card>
        </aside>
      </div>
    </main>
  );
}
