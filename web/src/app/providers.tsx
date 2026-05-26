import type { ReactNode } from "react";
import { SessionProvider } from "../auth/session";
import { FeedbackProvider } from "./feedback";

type AppProvidersProps = {
  children: ReactNode;
};

export function AppProviders({ children }: AppProvidersProps) {
  return (
    <SessionProvider>
      <FeedbackProvider>{children}</FeedbackProvider>
    </SessionProvider>
  );
}
