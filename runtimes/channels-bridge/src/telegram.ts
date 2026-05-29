import { Bot, Context } from "grammy";
import type { ChannelProvider, SendFunc } from "./types.js";

export interface TelegramConfig {
  token: string;
  allowedUsers: string[];
}

/**
 * Telegram channel provider using grammy.
 * Long-polls for messages, filters by allowed users,
 * and streams agent responses by editing messages.
 */
export class TelegramChannel implements ChannelProvider {
  private bot: Bot;
  private config: TelegramConfig;
  private send: SendFunc | null = null;
  private pending = new Map<string, { text: string; messageId: number }>();

  constructor(config: TelegramConfig) {
    this.config = config;
    this.bot = new Bot(config.token);
  }

  async start(send: SendFunc): Promise<void> {
    this.send = send;

    // Register message handler
    this.bot.on("message:text", (ctx) => this.handleMessage(ctx));

    // Start long-polling (non-blocking)
    this.bot.start({
      onStart: () => {
        console.log(
          `[telegram] bot started (allowed: ${this.config.allowedUsers.join(", ") || "*"})`
        );
      },
    });
  }

  async stop(): Promise<void> {
    await this.bot.stop();
    console.log("[telegram] bot stopped");
  }

  async sendResponse(chatId: string, text: string): Promise<void> {
    const numericChatId = Number(chatId);
    const existing = this.pending.get(chatId);

    if (!existing) {
      // Send initial message
      try {
        const msg = await this.bot.api.sendMessage(numericChatId, text);
        this.pending.set(chatId, { text, messageId: msg.message_id });
      } catch (err) {
        console.error("[telegram] sendMessage failed:", err);
      }
    } else {
      // Accumulate and edit
      existing.text += text;
      try {
        await this.bot.api.editMessageText(
          numericChatId,
          existing.messageId,
          existing.text
        );
      } catch {
        // Edit can fail if text hasn't changed enough — ignore
      }
    }
  }

  async completeResponse(chatId: string): Promise<void> {
    this.pending.delete(chatId);
  }

  private handleMessage(ctx: Context): void {
    const from = ctx.from;
    const chatId = ctx.chat?.id?.toString();
    const text = ctx.message?.text;

    if (!from || !chatId || !text) return;

    // Check allowed users
    if (!this.isAllowed(from.id, from.username)) {
      console.warn(
        `[telegram] ignoring unauthorized user: @${from.username} (${from.id})`
      );
      return;
    }

    console.log(
      `[telegram] message from @${from.username} in ${chatId} (${text.length} chars)`
    );

    this.send?.(chatId, text);
  }

  /** Check if a user is allowed based on numeric ID or username. */
  isAllowed(userId: number, username?: string): boolean {
    if (this.config.allowedUsers.length === 0) {
      return true; // No filter = allow all
    }

    const userIdStr = userId.toString();
    const normalizedUsername = (username ?? "").toLowerCase();

    return this.config.allowedUsers.some((allowed) => {
      const trimmed = allowed.trim();
      if (!trimmed) return false;

      // Match by numeric ID
      if (trimmed === userIdStr) return true;

      // Match by @username or bare username (case-insensitive)
      const normalized = trimmed.replace(/^@/, "").toLowerCase();
      return normalized === normalizedUsername;
    });
  }
}
