import ConfigProvider from "antd/es/config-provider";
import type { ReactNode } from "react";
import { SessionProvider } from "../auth/session";
import { paperMindAntTheme } from "./antd-theme";
import { FeedbackProvider } from "./feedback";

type AppProvidersProps = {
  children: ReactNode;
};

export function AppProviders({ children }: AppProvidersProps) {
  return (
    <ConfigProvider theme={paperMindAntTheme}>
      <SessionProvider>
        <FeedbackProvider>{children}</FeedbackProvider>
      </SessionProvider>
    </ConfigProvider>
  );
}
