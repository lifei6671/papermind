import { X } from "lucide-react";
import { useCallback, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { FeedbackContext } from "./feedback-context";
import type { FeedbackMessage, FeedbackTone } from "./feedback-context";

type FeedbackProviderProps = {
  children: ReactNode;
};

export function FeedbackProvider({ children }: FeedbackProviderProps) {
  const [message, setMessage] = useState<FeedbackMessage | null>(null);

  const show = useCallback((tone: FeedbackTone, text: string) => {
    setMessage({ id: Date.now(), tone, text });
  }, []);

  const clear = useCallback(() => {
    setMessage(null);
  }, []);

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
      {message && (
        <div className={`feedback-toast feedback-toast--${message.tone}`} role="alert">
          <span>{message.text}</span>
          <button aria-label="关闭提示" onClick={clear} type="button">
            <X aria-hidden="true" size={16} />
          </button>
        </div>
      )}
    </FeedbackContext.Provider>
  );
}
