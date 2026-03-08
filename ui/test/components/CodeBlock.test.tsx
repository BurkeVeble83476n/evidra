import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { CodeBlock } from "../../src/components/CodeBlock";

describe("CodeBlock", () => {
  beforeEach(() => {
    Object.assign(navigator, {
      clipboard: { writeText: vi.fn().mockResolvedValue(undefined) },
    });
  });

  it("renders code content", () => {
    render(<CodeBlock code="echo hello" />);
    expect(screen.getByText("echo hello")).toBeInTheDocument();
  });

  it("copies code to clipboard on button click", async () => {
    render(<CodeBlock code="echo hello" />);
    const btn = screen.getByRole("button", { name: /copy/i });
    await userEvent.click(btn);
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith("echo hello");
  });
});
