import { createRootRoute } from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/react-router-devtools";
import { AppProvider } from "../context/AppContext";
import { Layout } from "../components/Layout";

const RootLayout = () => (
  <AppProvider>
    <Layout />
    <TanStackRouterDevtools />
  </AppProvider>
);

export const Route = createRootRoute({ component: RootLayout });
