import "@testing-library/jest-dom/vitest";

const createStorage = (): Storage => {
  let store: Record<string, string> = {};
  return {
    getItem: (key: string) => store[key] ?? null,
    setItem: (key: string, value: string) => {
      store[key] = String(value);
    },
    removeItem: (key: string) => {
      delete store[key];
    },
    clear: () => {
      store = {};
    },
    get length() {
      return Object.keys(store).length;
    },
    key: (index: number) => Object.keys(store)[index] ?? null,
  };
};

Object.defineProperty(globalThis, "localStorage", { value: createStorage() });
Object.defineProperty(globalThis, "sessionStorage", {
  value: createStorage(),
});

// Mock SVGElement.getBBox for mermaid (not available in jsdom)
if (typeof SVGElement !== "undefined") {
  SVGElement.prototype.getBBox = () => ({
    x: 0,
    y: 0,
    width: 0,
    height: 0,
  });
}

// Suppress mermaid unhandled rejections in jsdom (getBBox-related)
process.on("unhandledRejection", () => {});

// Mock window.matchMedia for components that check prefers-color-scheme
Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  }),
});
