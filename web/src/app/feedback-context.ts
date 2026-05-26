import { createContext, useContext } from "react";

export type FeedbackTone = "error" | "success";

export type FeedbackMessage = {
  id: number;
  tone: FeedbackTone;
  text: string;
};

export type FeedbackContextValue = {
  showError: (message: string) => void;
  showSuccess: (message: string) => void;
  clear: () => void;
};

export const FeedbackContext = createContext<FeedbackContextValue | null>(null);

export function useFeedback() {
  const value = useContext(FeedbackContext);
  if (!value) {
    throw new Error("useFeedback must be used within FeedbackProvider");
  }

  return value;
}
