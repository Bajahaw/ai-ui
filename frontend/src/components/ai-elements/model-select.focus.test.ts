import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ModelSelect } from "./model-select";

const models = [
  "alpha-chat",
  "opencode/mimo-v2.6-flash-free",
  "opencode/mimo-v2.5-free",
  "opencode/mimo-v2-omni-free",
  "opencode/mimo-v2-flash-free",
].map((name) => ({ id: `bajahaw-e1e1/${name}`, name, provider: "bajahaw-e1e1" }));

let container: HTMLDivElement;
let root: Root;
let onChange: ReturnType<typeof vi.fn<(value: string) => void>>;

beforeEach(async () => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.spyOn(HTMLElement.prototype, "offsetHeight", "get").mockImplementation(function (this: HTMLElement) {
    return this.hasAttribute("data-radix-select-viewport") ? 280 : 40;
  });
  vi.spyOn(HTMLElement.prototype, "offsetWidth", "get").mockReturnValue(320);
  vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockReturnValue(280);
  vi.spyOn(Element.prototype, "scrollHeight", "get").mockImplementation(function (this: HTMLElement) {
    const rows = this.querySelector<HTMLElement>('[data-slot="model-options"]');
    return 73 + Array.from(rows?.children ?? []).reduce((height, child) =>
      height + (child.getAttribute("role") === "option" ? 40 : parseFloat((child as HTMLElement).style.height) || 0), 0);
  });
  Object.defineProperty(HTMLElement.prototype, "scrollTo", {
    configurable: true,
    value: function (this: HTMLElement, options: ScrollToOptions) {
      this.scrollTop = options.top ?? 0;
      this.dispatchEvent(new Event("scroll"));
    },
  });
  Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
    configurable: true,
    value: vi.fn(function (this: HTMLElement) {
      const viewport = this.closest<HTMLElement>("[data-radix-select-viewport]");
      if (!viewport || this.dataset.modelIndex === undefined) return;
      const top = 40 + Number(this.dataset.modelIndex) * 40;
      if (top < viewport.scrollTop + 40) viewport.scrollTop = top - 40;
      else if (top + 40 > viewport.scrollTop + 280) viewport.scrollTop = top + 40 - 280;
      viewport.dispatchEvent(new Event("scroll"));
    }),
  });
  container = document.createElement("div");
  document.body.append(container);
  root = createRoot(container);
  onChange = vi.fn();
  await act(async () => {
    root.render(createElement(ModelSelect, {
      models,
      value: models[0].id,
      onChange,
      showCount: true,
    }));
  });
  await act(async () => {
    container.querySelector("button")!.dispatchEvent(new KeyboardEvent("keydown", {
      key: "Enter",
      bubbles: true,
    }));
  });
});

afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  Reflect.deleteProperty(HTMLElement.prototype, "scrollIntoView");
  Reflect.deleteProperty(HTMLElement.prototype, "scrollTo");
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

function input() {
  return document.querySelector<HTMLInputElement>('input[aria-label="Search models"]')!;
}

function options() {
  return Array.from(document.querySelectorAll<HTMLElement>('[role="option"]'));
}

async function changeSearch(value: string, caret = value.length) {
  const field = input();
  await act(async () => {
    field.focus();
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!;
    setter.call(field, value);
    field.setSelectionRange(caret, caret);
    field.dispatchEvent(new Event("input", { bubbles: true }));
  });
  expect(document.activeElement).toBe(field);
  expect(field.selectionStart).toBe(caret);
  expect(field.selectionEnd).toBe(caret);
}

async function key(target: Element, value: string) {
  await act(async () => {
    target.dispatchEvent(new KeyboardEvent("keydown", {
      key: value,
      bubbles: true,
      cancelable: true,
    }));
  });
}

function pointer(type: string) {
  const event = new MouseEvent(type, { button: 0, bubbles: true, cancelable: true });
  Object.defineProperty(event, "pointerType", { value: "mouse" });
  Object.defineProperty(event, "pointerId", { value: 1 });
  return event;
}

const manyModels = Array.from({ length: 1000 }, (_, index) => ({
  id: `model-${index}`,
  name: `Model ${String(index).padStart(4, "0")}`,
  provider: "bajahaw-e1e1",
}));

async function renderMany(value: string | undefined = manyModels[0].id) {
  await act(async () => {
    root.render(createElement(ModelSelect, {
      models: manyModels,
      value,
      onChange,
      showCount: true,
    }));
  });
}

function modelAt(index: number) {
  return document.querySelector<HTMLElement>(`[data-model-index="${index}"]`);
}

describe("model search focus", () => {
  it("mounts a bounded window of models and updates it when scrolling", async () => {
    await renderMany();
    expect(options().length).toBeLessThan(30);
    expect(modelAt(400)).toBeNull();
    const viewport = input().closest<HTMLElement>("[data-radix-select-viewport]")!;
    await act(async () => {
      viewport.scrollTop = 16040;
      viewport.dispatchEvent(new Event("scroll"));
    });
    expect(modelAt(400)).not.toBeNull();
    expect(options().length).toBeLessThan(30);
    expect(document.body.textContent).toContain("1000 models available");
  });

  it("navigates across virtual windows and selects an offscreen model", async () => {
    await renderMany();
    for (let index = 1; index <= 30; index++) {
      await key(document.activeElement!, "ArrowDown");
      expect(document.activeElement).toBe(modelAt(index));
    }
    expect(options().length).toBeLessThan(30);
    await key(document.activeElement!, "Enter");
    expect(onChange).toHaveBeenCalledExactlyOnceWith(manyModels[30].id);
  });

  it("supports Home, End, and paging across unmounted models", async () => {
    await renderMany();
    await key(document.activeElement!, "End");
    expect(document.activeElement).toBe(modelAt(999));
    await key(document.activeElement!, "Home");
    expect(document.activeElement).toBe(modelAt(0));
    await key(document.activeElement!, "PageDown");
    expect(document.activeElement).toBe(modelAt(6));
    await key(document.activeElement!, "PageUp");
    expect(document.activeElement).toBe(modelAt(0));
  });

  it("mounts and focuses a selected model outside the initial window without focusing search", async () => {
    await renderMany(manyModels[700].id);
    expect(document.activeElement).toBe(modelAt(700));
    expect(document.activeElement).not.toBe(input());
    expect(options().length).toBeLessThan(30);
  });

  it("searches all models and preserves the cursor when filtering a scrolled list", async () => {
    await renderMany();
    await key(document.activeElement!, "End");
    await changeSearch("0998");
    expect(options()).toHaveLength(1);
    expect(options()[0].textContent).toContain("Model 0998");
    await changeSearch("");
    expect(options().length).toBeLessThan(30);
    expect(modelAt(1)).not.toBeNull();
  });

  it("does not autofocus search on opening or reopening the picker", async () => {
    expect(document.activeElement).not.toBe(input());
    expect(document.activeElement).toBe(options()[0]);
    await changeSearch("m");
    await key(input(), "Escape");
    await key(container.querySelector("button")!, "Enter");
    expect(input().value).toBe("");
    expect(document.activeElement).not.toBe(input());
    expect(document.activeElement).toBe(options()[0]);
  });

  it("uses sidebar-style underline search with row padding and a scrolling footer", () => {
    const field = input();
    const header = field.closest<HTMLElement>(".sticky")!;
    expect(header).not.toBeNull();
    expect(header.classList.contains("px-3")).toBe(true);
    expect(header.classList.contains("py-2.5")).toBe(true);
    expect(field.classList.contains("border-0")).toBe(true);
    expect(field.classList.contains("border-b")).toBe(true);
    expect(field.classList.contains("focus:border-primary")).toBe(true);
    expect(field.classList.contains("focus-visible:border-primary")).toBe(true);
    expect(field.classList.contains("border-primary")).toBe(false);
    expect(field.classList.contains("rounded-none")).toBe(true);
    expect(field.classList.contains("shadow-none")).toBe(true);
    expect(field.classList.contains("h-9")).toBe(true);
    expect(field.classList.contains("-translate-y-1/2")).toBe(true);
    expect(field.parentElement!.classList.contains("translate-y-[-2px]")).toBe(true);
    expect(options()[0].classList.contains("py-2.5")).toBe(true);
    const footer = Array.from(document.querySelectorAll("div"))
      .find((element) => element.textContent?.trim() === "5 models available")!;
    expect(footer.parentElement).toBe(header.parentElement);
    expect(footer.parentElement!.lastElementChild).toBe(footer);
    expect(footer.closest(".sticky")).toBeNull();
  });

  it("keeps focus and the caret when the first letter removes the selected option", async () => {
    const field = input();
    await act(async () => field.focus());
    expect(document.activeElement).toBe(field);
    await key(field, "m");
    await changeSearch("m");
    expect(options()).toHaveLength(4);
    expect(input()).toBe(field);
    await changeSearch("mi");
    await changeSearch("mimo free");
    expect(options()).toHaveLength(4);
    expect(container.textContent).toContain("alpha-chat");
  });

  it("keeps focus when deleting the first letter restores the selected option", async () => {
    await changeSearch("m");
    await key(input(), "Backspace");
    await changeSearch("");
    expect(options()).toHaveLength(5);
  });

  it("keeps focus through empty results and clearing the query", async () => {
    await changeSearch("z");
    expect(options()).toHaveLength(0);
    expect(document.body.textContent).toContain("No models match your search.");
    await changeSearch("");
    expect(options()).toHaveLength(5);
  });

  it("preserves a caret in the middle of the query while results change", async () => {
    await changeSearch("mimo free");
    await changeSearch("mimo fxree", 7);
    expect(options()).toHaveLength(0);
    await changeSearch("mimo free", 6);
    expect(options()).toHaveLength(4);
  });

  it("allows arrow-key navigation and Enter selection", async () => {
    await changeSearch("mimo free");
    await key(input(), "ArrowDown");
    expect(document.activeElement).toBe(options()[0]);
    await key(options()[0], "Enter");
    expect(onChange).toHaveBeenCalledExactlyOnceWith(models[1].id);
    expect(document.querySelector('[role="listbox"]')).toBeNull();
  });

  it("allows pointer selection while search has focus", async () => {
    await changeSearch("mimo omni");
    const option = options()[0];
    await act(async () => {
      option.dispatchEvent(pointer("pointerdown"));
      option.focus();
    });
    expect(document.activeElement).toBe(option);
    await act(async () => {
      option.dispatchEvent(pointer("pointerup"));
    });
    expect(onChange).toHaveBeenCalledExactlyOnceWith(models[3].id);
  });

  it("protects search focus after returning from the options and during pointer movement", async () => {
    await act(async () => input().focus());
    await key(input(), "ArrowDown");
    expect(document.activeElement).toBe(options()[0]);
    await act(async () => input().focus());
    await changeSearch("m");
    await act(async () => {
      options()[0].dispatchEvent(pointer("pointermove"));
    });
    expect(document.activeElement).toBe(input());
    await changeSearch("");
  });
});
