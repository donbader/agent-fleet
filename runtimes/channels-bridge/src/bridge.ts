import { spawn, ChildProcess } from "node:child_process";
import { createInterface } from "node:readline";
import type { ACPMessage, BridgeConfig, SendFunc } from "./types.js";

/**
 * Bridge routes messages between a channel provider and an agent process.
 * The agent communicates via ACP (ndJSON over stdin/stdout).
 */
export class Bridge {
  private agent: ChildProcess | null = null;
  private sessions = new Map<string, string>(); // chatId → sessionId
  private config: BridgeConfig;

  constructor(config: BridgeConfig) {
    this.config = config;
  }

  /** Start the bridge: spawn agent, start channel, route messages. */
  async run(signal: AbortSignal): Promise<void> {
    this.spawnAgent();
    console.log(`[bridge] agent spawned: ${this.config.agentCmd} (pid ${this.agent?.pid})`);

    // Read agent output in background
    this.readAgentOutput();

    // Start channel with our message handler
    const send: SendFunc = (chatId, text) => this.handleUserMessage(chatId, text);
    await this.config.channel.start(send);
    console.log("[bridge] channel started, bridge is ready");

    // Wait for shutdown signal
    await new Promise<void>((resolve) => {
      signal.addEventListener("abort", () => {
        console.log("[bridge] shutting down...");
        resolve();
      });
    });

    await this.shutdown();
  }

  /** Handle an incoming user message from the channel. */
  private handleUserMessage(chatId: string, text: string): void {
    let sessionId = this.sessions.get(chatId);
    const isNew = !sessionId;

    if (!sessionId) {
      sessionId = chatId;
      this.sessions.set(chatId, sessionId);
    }

    // Send session.start for new sessions
    if (isNew) {
      this.sendToAgent({ type: "session.start", session_id: sessionId });
    }

    // Send the user message
    this.sendToAgent({
      type: "message.send",
      session_id: sessionId,
      content: text,
    });
  }

  /** Route an agent response back to the appropriate chat. */
  private async routeAgentMessage(msg: ACPMessage): Promise<void> {
    if (!msg.session_id) return;

    // Find chatId for this session
    let chatId: string | undefined;
    for (const [cid, sid] of this.sessions) {
      if (sid === msg.session_id) {
        chatId = cid;
        break;
      }
    }

    if (!chatId) {
      console.warn(`[bridge] no chat for session ${msg.session_id}`);
      return;
    }

    switch (msg.type) {
      case "message.delta":
        if (msg.delta) {
          await this.config.channel.sendResponse(chatId, msg.delta);
        }
        break;
      case "message.complete":
        await this.config.channel.completeResponse(chatId);
        break;
    }
  }

  private spawnAgent(): void {
    this.agent = spawn("sh", ["-c", this.config.agentCmd], {
      stdio: ["pipe", "pipe", "inherit"],
    });

    this.agent.on("exit", (code) => {
      console.log(`[bridge] agent exited with code ${code}`);
    });
  }

  private readAgentOutput(): void {
    if (!this.agent?.stdout) return;

    const rl = createInterface({ input: this.agent.stdout });
    rl.on("line", (line) => {
      try {
        const msg: ACPMessage = JSON.parse(line);
        this.routeAgentMessage(msg);
      } catch {
        console.warn("[bridge] invalid ACP message from agent:", line);
      }
    });
  }

  private sendToAgent(msg: ACPMessage): void {
    if (!this.agent?.stdin?.writable) {
      console.error("[bridge] agent stdin not writable");
      return;
    }
    this.agent.stdin.write(JSON.stringify(msg) + "\n");
  }

  private async shutdown(): Promise<void> {
    await this.config.channel.stop();
    if (this.agent) {
      this.agent.kill("SIGTERM");
      // Give agent time to exit gracefully
      await new Promise((resolve) => setTimeout(resolve, 2000));
      if (!this.agent.killed) {
        this.agent.kill("SIGKILL");
      }
    }
  }
}
