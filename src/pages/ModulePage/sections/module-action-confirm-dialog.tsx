import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";

type ModuleActionConfirmDialogProps = {
  open: boolean;
  title: string;
  description: string;
  confirmLabel: string;
  running?: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
};

export function ModuleActionConfirmDialog({
  open,
  title,
  description,
  confirmLabel,
  running = false,
  onOpenChange,
  onConfirm,
}: ModuleActionConfirmDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={running}>Cancel</AlertDialogCancel>
          <AlertDialogAction onClick={onConfirm} disabled={running}>
            {running ? "Running..." : confirmLabel}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
