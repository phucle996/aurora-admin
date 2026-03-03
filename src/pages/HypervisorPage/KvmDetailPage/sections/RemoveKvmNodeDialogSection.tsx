import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { KvmTerminalStreamSection } from "@/components/terminal-stream";

type RemoveKvmNodeDialogSectionProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  nodeName: string;
  confirmValue: string;
  onConfirmValueChange: (value: string) => void;
  loading: boolean;
  logs: string[];
  onConfirm: () => void;
  isDark: boolean;
};

export function RemoveKvmNodeDialogSection({
  open,
  onOpenChange,
  nodeName,
  confirmValue,
  onConfirmValueChange,
  loading,
  logs,
  onConfirm,
  isDark,
}: RemoveKvmNodeDialogSectionProps) {
  const expected = nodeName.trim();
  const canConfirm =
    !loading &&
    expected.length > 0 &&
    confirmValue.trim().length > 0 &&
    confirmValue.trim() === expected;
  const showTerminal = loading || logs.length > 0;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={!loading}
        className={cn(
          "w-[96vw] max-w-[96vw] sm:max-w-5xl",
          isDark && "border-white/15 bg-slate-950/95",
        )}
        onInteractOutside={(event) => {
          if (loading) {
            event.preventDefault();
          }
        }}
        onEscapeKeyDown={(event) => {
          if (loading) {
            event.preventDefault();
          }
        }}
      >
        <DialogHeader>
          <DialogTitle>Remove KVM Node</DialogTitle>
          <DialogDescription>
            Muon remove node nay khoi Aurora? Nhap dung ten node de xac nhan.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-2">
          <p className="text-sm">
            Node name:{" "}
            <span className="font-semibold">
              {expected || "(unknown node)"}
            </span>
          </p>
          <Input
            value={confirmValue}
            onChange={(event) => onConfirmValueChange(event.target.value)}
            placeholder={
              expected ? `Nhap ten node: ${expected}` : "Nhap ten node"
            }
            disabled={loading}
          />
        </div>

        {showTerminal && (
          <KvmTerminalStreamSection
            logs={logs}
            checking={loading}
            terminalLabel="ubuntu@kvm-remove:~"
            shellPrompt="phucle@kvm-node:~$"
            shellName="bash"
            emptyMessage="waiting for remove command..."
            className="mt-1"
          />
        )}

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={loading}
          >
            Cancel
          </Button>
          <Button
            type="button"
            variant="destructive"
            onClick={onConfirm}
            disabled={!canConfirm}
          >
            {loading ? "Removing..." : "Remove node"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
