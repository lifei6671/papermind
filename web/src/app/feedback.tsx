import { Button } from "../components/ui/Button";
import { X } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";
import { FeedbackContext } from "./feedback-context";
import type { FeedbackMessage, FeedbackTone } from "./feedback-context";

type FeedbackProviderProps = {
  children: ReactNode;
};

const feedbackVisibleDuration = 5000;

export function FeedbackProvider({ children }: FeedbackProviderProps) {
  const [messages, setMessages] = useState<FeedbackMessage[]>([]);
  const nextMessageID = useRef(1);
  const timers = useRef(new Map<number, number>());

  const dismiss = useCallback((id: number) => {
    const timer = timers.current.get(id);
    if (timer) {
      window.clearTimeout(timer);
      timers.current.delete(id);
    }
    setMessages((items) => items.filter((item) => item.id !== id));
  }, []);

  const show = useCallback((tone: FeedbackTone, text: string) => {
    const id = nextMessageID.current;
    nextMessageID.current += 1;
    setMessages((items) => [...items, { id, tone, text }]);
    const timer = window.setTimeout(() => {
      timers.current.delete(id);
      setMessages((items) => items.filter((item) => item.id !== id));
    }, feedbackVisibleDuration);
    timers.current.set(id, timer);
  }, []);

  const clear = useCallback(() => {
    timers.current.forEach((timer) => window.clearTimeout(timer));
    timers.current.clear();
    setMessages([]);
  }, []);

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
      {children}
      {messages.length > 0 && (
        <div aria-live="polite" className="feedback-toast-stack">
          {messages.map((message) => (
            <div className={`feedback-toast feedback-toast--${message.tone}`} key={message.id} role="alert">
              <span>{message.text}</span>
              <Button aria-label="关闭提示" onClick={() => dismiss(message.id)} type="button">
                <X aria-hidden="true" size={16} />
              </Button>
            </div>
          ))}
        </div>
      )}
    </FeedbackContext.Provider>
  );
}
