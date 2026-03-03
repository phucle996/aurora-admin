import { CheckCircle2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

type KvmFeedbackSectionProps = {
  createdMessage: string | null;
  textPrimary: string;
  panelClass: string;
  onCloseCreated: () => void;
};

export function KvmFeedbackSection({
  createdMessage,
  textPrimary,
  panelClass,
  onCloseCreated,
}: KvmFeedbackSectionProps) {
  return (
    <>
      {createdMessage && (
        <Card className={cn("border-emerald-400/30 shadow-lg", panelClass)}>
          <CardContent className="flex items-center justify-between gap-3 pt-5">
            <div className="flex items-center gap-2">
              <CheckCircle2 className="h-4 w-4 text-emerald-500" />
              <p className={cn("text-sm", textPrimary)}>{createdMessage}</p>
            </div>
            <Button
              variant="outline"
              className={cn("h-8")}
              onClick={onCloseCreated}
            >
              Close
            </Button>
          </CardContent>
        </Card>
      )}
    </>
  );
}
