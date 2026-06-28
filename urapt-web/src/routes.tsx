import { lazy, Suspense } from "react";
import { createBrowserRouter } from "react-router-dom";
import { RootGuard } from "@/components/common/root-guard";
import { AdminRoute } from "@/components/common/admin-route";
import { NotFound } from "@/components/common/not-found";
import { FullPageSpinner } from "@/components/common/spinner";
import { ReposList } from "@/features/repos/ReposList";

const RepoDetail = lazy(() =>
  import("@/features/repos/RepoDetail").then((m) => ({ default: m.RepoDetail })),
);
const Tokens = lazy(() =>
  import("@/features/tokens/Tokens").then((m) => ({ default: m.Tokens })),
);
const Users = lazy(() =>
  import("@/features/users/Users").then((m) => ({ default: m.Users })),
);
const Settings = lazy(() =>
  import("@/features/settings/Settings").then((m) => ({ default: m.Settings })),
);
const Login = lazy(() =>
  import("@/features/auth/Login").then((m) => ({ default: m.Login })),
);
const FirstRun = lazy(() =>
  import("@/features/setup/FirstRun").then((m) => ({ default: m.FirstRun })),
);

const fallback = <FullPageSpinner />;

export const router = createBrowserRouter([
  {
    path: "/login",
    element: (
      <Suspense fallback={fallback}>
        <Login />
      </Suspense>
    ),
  },
  {
    path: "/setup",
    element: (
      <Suspense fallback={fallback}>
        <FirstRun />
      </Suspense>
    ),
  },
  {
    path: "/",
    element: <RootGuard />,
    children: [
      { index: true, element: <ReposList /> },
      {
        path: "repos/:repo",
        element: (
          <Suspense fallback={fallback}>
            <RepoDetail />
          </Suspense>
        ),
      },
      {
        path: "repos/:repo/:tab",
        element: (
          <Suspense fallback={fallback}>
            <RepoDetail />
          </Suspense>
        ),
      },
      {
        path: "tokens",
        element: (
          <Suspense fallback={fallback}>
            <Tokens />
          </Suspense>
        ),
      },
      {
        path: "users",
        element: (
          <AdminRoute>
            <Suspense fallback={fallback}>
              <Users />
            </Suspense>
          </AdminRoute>
        ),
      },
      {
        path: "settings",
        element: (
          <Suspense fallback={fallback}>
            <Settings />
          </Suspense>
        ),
      },
      { path: "*", element: <NotFound /> },
    ],
  },
]);
