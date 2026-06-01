export const refreshFeedbackMinDurationMs = 900;

export async function withRefreshFeedback<T>(
  operation: Promise<T>,
  minDurationMs = refreshFeedbackMinDurationMs,
) {
  const feedbackTimer = new Promise((resolve) => {
    window.setTimeout(resolve, minDurationMs);
  });

  try {
    const result = await operation;
    await feedbackTimer;
    return result;
  } catch (error) {
    await feedbackTimer;
    throw error;
  }
}
