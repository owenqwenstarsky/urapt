import { NavLink as RRNavLink, Outlet } from "react-router-dom";
import {
  LogOut,
  Package,
  Settings,
  Terminal,
  User as UserIcon,
  Users,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { ThemeToggle } from "@/components/common/theme-toggle";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/stores/auth";
import { useMe, useServerInfo } from "@/hooks/use-server";
import { useLogout } from "@/hooks/use-logout";

interface NavItem {
  to: string;
  label: string;
  icon: typeof Package;
  adminOnly?: boolean;
}

const NAV: NavItem[] = [
  { to: "/", label: "Repositories", icon: Package },
  { to: "/tokens", label: "Tokens", icon: Terminal },
  { to: "/users", label: "Users", icon: Users, adminOnly: true },
  { to: "/settings", label: "Settings", icon: Settings },
];

export function AppShell() {
  const signOut = useLogout();
  const token = useAuthStore((s) => s.token);
  const me = useMe(!!token);
  const serverInfo = useServerInfo();

  const user = me.data;
  const isAdmin = user?.is_admin ?? false;

  return (
    <div className="flex min-h-dvh">
      <aside className="hidden w-60 shrink-0 flex-col border-r bg-card md:flex">
        <div className="flex h-14 items-center gap-2 border-b px-5">
          <div className="flex size-7 items-center justify-center rounded-md bg-primary text-primary-foreground">
            <Terminal className="size-4" />
          </div>
          <span className="font-semibold tracking-tight">urapt</span>
        </div>
        <nav className="flex-1 space-y-1 p-3">
          {NAV.filter((item) => !item.adminOnly || isAdmin).map((item) => (
            <NavLink key={item.to} item={item} />
          ))}
        </nav>
        <div className="border-t p-3 text-xs text-muted-foreground">
          {serverInfo.data ? <span>urapt v{serverInfo.data.version}</span> : null}
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 items-center justify-between gap-3 border-b px-4">
          <div className="flex items-center gap-2 md:hidden">
            <div className="flex size-7 items-center justify-center rounded-md bg-primary text-primary-foreground">
              <Terminal className="size-4" />
            </div>
            <span className="font-semibold tracking-tight">urapt</span>
          </div>
          <div className="hidden md:block" />
          <div className="flex items-center gap-1">
            <ThemeToggle />
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="sm" className="gap-2">
                  <Avatar className="size-6">
                    <AvatarFallback className="text-xs">
                      {user?.username?.slice(0, 2).toUpperCase() ?? "?"}
                    </AvatarFallback>
                  </Avatar>
                  <span className="max-w-[10rem] truncate">{user?.username ?? "…"}</span>
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-48">
                <DropdownMenuLabel className="flex items-center gap-2">
                  <UserIcon className="size-4" />
                  <span className="truncate">{user?.username}</span>
                  {isAdmin ? (
                    <span className="ml-auto rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium">
                      admin
                    </span>
                  ) : null}
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={signOut}>
                  <LogOut className="size-4" />
                  Sign out
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </header>

        <main className="flex-1 overflow-y-auto bg-muted/30 p-4 md:p-6">
          {/* Mobile nav */}
          <nav className="mb-4 flex gap-1 overflow-x-auto md:hidden">
            {NAV.filter((item) => !item.adminOnly || isAdmin).map((item) => (
              <NavLink key={item.to} item={item} compact />
            ))}
          </nav>
          <div className="mx-auto max-w-5xl">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  );
}

function NavLink({ item, compact }: { item: NavItem; compact?: boolean }) {
  const Icon = item.icon;
  return (
    <RRNavLink
      to={item.to}
      end={item.to === "/"}
      className={({ isActive }) =>
        cn(
          "flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors",
          "text-muted-foreground hover:bg-accent hover:text-foreground",
          isActive && "bg-accent text-foreground",
          compact && "shrink-0",
        )
      }
    >
      <Icon className="size-4" />
      {item.label}
    </RRNavLink>
  );
}
