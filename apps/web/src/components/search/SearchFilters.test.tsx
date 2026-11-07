import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { buildSearchQuery } from "@/lib/search";
import { SearchFilters } from "./SearchFilters";

const push = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push, replace: vi.fn(), prefetch: vi.fn() }),
}));

const props = {
  categories: [
    { value: "namkeen", label: "Namkeen" },
    { value: "tiles", label: "Tiles" },
  ],
  states: [{ value: "gujarat", label: "Gujarat" }],
  roles: [{ value: "manufacturer", label: "Manufacturer" }],
};

describe("buildSearchQuery", () => {
  it("drops empty values and trims", () => {
    expect(buildSearchQuery({ q: "  cnc ", category: "", state: "gujarat", city: " ", role: "manufacturer" })).toBe("q=cnc&state=gujarat&role=manufacturer");
    expect(buildSearchQuery({})).toBe("");
  });
});

describe("SearchFilters", () => {
  beforeEach(() => push.mockReset());

  it("pre-fills from the current filters", () => {
    render(<SearchFilters {...props} initial={{ q: "bhujia", category: "namkeen" }} />);
    expect(screen.getByLabelText("Keyword")).toHaveValue("bhujia");
    expect(screen.getByLabelText("Category")).toHaveValue("namkeen");
    expect(screen.getByLabelText("State")).toHaveValue("");
  });

  it("navigates to /search with only the chosen filters", async () => {
    const user = userEvent.setup();
    render(<SearchFilters {...props} initial={{}} />);
    await user.type(screen.getByLabelText("Keyword"), "CNC");
    await user.selectOptions(screen.getByLabelText("State"), "gujarat");
    await user.selectOptions(screen.getByLabelText("Business role"), "manufacturer");
    await user.type(screen.getByLabelText("City"), "Rajkot");
    await user.click(screen.getByRole("button", { name: "Search" }));
    expect(push).toHaveBeenCalledWith("/search?q=CNC&state=gujarat&city=Rajkot&role=manufacturer");
  });

  it("clears all filters", async () => {
    const user = userEvent.setup();
    render(<SearchFilters {...props} initial={{ q: "x", state: "gujarat" }} />);
    await user.click(screen.getByRole("button", { name: "Clear filters" }));
    expect(push).toHaveBeenCalledWith("/search");
    expect(screen.getByLabelText("Keyword")).toHaveValue("");
  });
});
