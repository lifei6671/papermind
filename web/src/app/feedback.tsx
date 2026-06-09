import message from "antd/es/message";
import { useCallback, useEffect, useMemo, useRef } from "react";
import type { ReactNode } from "react";
import { FeedbackContext } from "./feedback-context";
import type { FeedbackTone } from "./feedback-context";

type FeedbackProviderProps = {
  children: ReactNode;
};

const feedbackVisibleDurationSeconds = 5;
export function FeedbackProvider({ children }: FeedbackProviderProps) {
  const pendingTimers = useRef<number[]>([]);
  const [messageApi, contextHolder] = message.useMessage({
    duration: feedbackVisibleDurationSeconds,
    top: 24,
    transitionName: "",
    classNames: {
      root: "feedback-message-root",
      title: "feedback-message-title",
    },
  });

  const show = useCallback((tone: FeedbackTone, text: string) => {
    const timer = window.setTimeout(() => {
      pendingTimers.current = pendingTimers.current.filter((item) => item !== timer);
      messageApi.open({
        content: <span>{text}</span>,
        type: tone,
      });
    }, 0);
    pendingTimers.current.push(timer);
  }, [messageApi]);

  const clear = useCallback(() => {
    pendingTimers.current.forEach((timer) => window.clearTimeout(timer));
    pendingTimers.current = [];
    messageApi.destroy();
  }, [messageApi]);

  useEffect(() => clear, [clear]);

  const value = useMemo(
    () => ({
      clear,
      showError: (text: string) => show("error", text),
      showSuccess: (text: string) => show("success", text),
    }),
    [clear, show],
  );

  return (
    <FeedbackContext.Provider value={value}>
      {contextHolder}
      {children}
    </FeedbackContext.Provider>
  );
}
