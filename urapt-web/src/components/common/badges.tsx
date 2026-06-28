import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { Access, Visibility } from "@/lib/api/types";

export function VisibilityBadge({ visibility }: { visibility: Visibility }) {
  return (
    <Badge
      variant={visibility === "public" ? "secondary" : "outline"}
      className="capitalize"
    >
      {visibility}
    </Badge>
  );
}

const ACCESS_STYLES: Record<Access, string> = {
  read: "bg-muted text-muted-foreground",
  write: "bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-300",
  "read-write": "bg-violet-100 text-violet-800 dark:bg-violet-950 dark:text-violet-300",
  admin: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
};

export function AccessBadge({ access }: { access: Access }) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium capitalize",
        ACCESS_STYLES[access],
      )}
    >
      {access}
    </span>
  );
}
