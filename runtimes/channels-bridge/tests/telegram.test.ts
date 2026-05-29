import { describe, it, expect } from "vitest";
import { TelegramChannel } from "../src/telegram.js";

describe("TelegramChannel.isAllowed", () => {
  it("allows all when no filter set", () => {
    const ch = new TelegramChannel({ token: "test", allowedUsers: [] });
    expect(ch.isAllowed(123, "anyone")).toBe(true);
  });

  it("matches by numeric ID", () => {
    const ch = new TelegramChannel({ token: "test", allowedUsers: ["12345"] });
    expect(ch.isAllowed(12345, "bob")).toBe(true);
    expect(ch.isAllowed(99999, "bob")).toBe(false);
  });

  it("matches by @username", () => {
    const ch = new TelegramChannel({ token: "test", allowedUsers: ["@alice"] });
    expect(ch.isAllowed(1, "alice")).toBe(true);
    expect(ch.isAllowed(1, "Alice")).toBe(true); // case-insensitive
    expect(ch.isAllowed(1, "bob")).toBe(false);
  });

  it("matches by bare username", () => {
    const ch = new TelegramChannel({
      token: "test",
      allowedUsers: ["charlie"],
    });
    expect(ch.isAllowed(1, "charlie")).toBe(true);
    expect(ch.isAllowed(1, "Charlie")).toBe(true);
  });

  it("denies when username is undefined", () => {
    const ch = new TelegramChannel({ token: "test", allowedUsers: ["@alice"] });
    expect(ch.isAllowed(999, undefined)).toBe(false);
  });

  it("handles multiple allowed users", () => {
    const ch = new TelegramChannel({
      token: "test",
      allowedUsers: ["@alice", "12345", "bob"],
    });
    expect(ch.isAllowed(1, "alice")).toBe(true);
    expect(ch.isAllowed(12345, "unknown")).toBe(true);
    expect(ch.isAllowed(2, "bob")).toBe(true);
    expect(ch.isAllowed(999, "eve")).toBe(false);
  });

  it("ignores empty entries", () => {
    const ch = new TelegramChannel({
      token: "test",
      allowedUsers: ["", " ", "@alice"],
    });
    expect(ch.isAllowed(1, "alice")).toBe(true);
    expect(ch.isAllowed(1, "bob")).toBe(false);
  });
});
