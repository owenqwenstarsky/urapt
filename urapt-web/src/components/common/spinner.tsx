import { Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";

export function Spinner({ className }: { className?: string }) {
  return <Loader2 className={cn("size-4 animate-spin", className)} />;
}

export function FullPageSpinner({ label }: { label?: string }) {
  return (
    <div className="flex min-h-dvh flex-col items-center justify-center gap-3">
      <Spinner className="size-6" />
      {label ? <p className="text-sm text-muted-foreground">{label}</p> : null}
    </div>
  );
}
