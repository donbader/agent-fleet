import { Bridge } from "./bridge.js";
import { TelegramChannel } from "./telegram.js";

const agentCmd = process.env.AGENT_CMD ?? "codex";
const botToken = process.env.TELEGRAM_BOT_TOKEN ?? "000000000:DUMMY";
const allowedUsersRaw = process.env.TELEGRAM_ALLOWED_USERS ?? "";
const allowedUsers = allowedUsersRaw
  .split(",")
  .map((s) => s.trim())
  .filter(Boolean);

console.log("[channels-bridge] starting", {
  agentCmd,
  allowedUsers,
});

const channel = new TelegramChannel({ token: botToken, allowedUsers });
const bridge = new Bridge({ agentCmd, channel });

const controller = new AbortController();

process.on("SIGINT", () => controller.abort());
process.on("SIGTERM", () => controller.abort());

bridge.run(controller.signal).catch((err) => {
  console.error("[channels-bridge] fatal:", err);
  process.exit(1);
});
