import { render, screen } from "@testing-library/react";
import { describe, it, expect, beforeEach } from "vitest";
import { App } from "../../src/App";

describe("App", () => {
  beforeEach(() => {
    document.documentElement.setAttribute("data-theme", "light");
    localStorage.clear();
  });

  it("renders the landing page with hero heading", () => {
    render(<App />);
    expect(
      screen.getByRole("heading", {
        name: /Behavioral Reliability for Infrastructure Automation/i,
      }),
    ).toBeInTheDocument();
  });

  it("renders navigation links", () => {
    render(<App />);
    const nav = screen.getByRole("navigation");
    expect(nav).toBeInTheDocument();
    expect(nav.querySelector('a[href="#features"]')).toHaveTextContent(
      "Features",
    );
    expect(nav.querySelector('a[href="#architecture"]')).toHaveTextContent(
      "Architecture",
    );
    expect(nav.querySelector('a[href="#get-started"]')).toHaveTextContent(
      "Get Started",
    );
  });
});
