import "@testing-library/jest-dom/vitest";

const storage = createMemoryStorage();
Object.defineProperty(window, "localStorage", {
  configurable: true,
  value: storage,
});
Object.defineProperty(globalThis, "localStorage", {
  configurable: true,
  value: storage,
});

function createMemoryStorage(): Storage {
  const values = new Map<string, string>();

  return {
    get length() {
      return values.size;
    },
    clear() {
      values.clear();
    },
    getItem(key) {
      return values.get(key) ?? null;
    },
    key(index) {
      return Array.from(values.keys())[index] ?? null;
    },
    removeItem(key) {
      values.delete(key);
    },
    setItem(key, value) {
      values.set(key, value);
    },
  };
}
