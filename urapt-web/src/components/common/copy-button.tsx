import { useState } from "react";
import { Check, Copy } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { cn, copyToClipboard } from "@/lib/utils";

interface CopyButtonProps {
  value: string;
  label?: string;
  className?: string;
  size?: "default" | "sm" | "icon";
  variant?: "default" | "outline" | "ghost" | "secondary";
}

export function CopyButton({
  value,
  label = "Copy",
  className,
  size = "sm",
  variant = "outline",
}: CopyButtonProps) {
  const [copied, setCopied] = useState(false);

  async function onCopy() {
    try {
      await copyToClipboard(value);
      setCopied(true);
      toast.success("Copied to clipboard");
      setTimeout(() => setCopied(false), 1500);
    } catch {
      toast.error("Couldn't copy to clipboard");
    }
  }

  return (
    <Button
      type="button"
      variant={variant}
      size={size}
      className={cn(className)}
      onClick={onCopy}
    >
      {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
      {size === "icon" ? null : <span>{copied ? "Copied" : label}</span>}
    </Button>
  );
}
