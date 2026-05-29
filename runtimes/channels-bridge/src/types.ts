/** ACP (Agent Client Protocol) message types. */
export interface ACPMessage {
  type: string;
  session_id?: string;
  content?: string;
  delta?: string;
  tool?: string;
  input?: unknown;
  output?: string;
}

/** Configuration for the bridge runtime. */
export interface BridgeConfig {
  agentCmd: string;
  channel: ChannelProvider;
}

/** Interface that channel implementations must satisfy. */
export interface ChannelProvider {
  start(send: SendFunc): Promise<void>;
  stop(): Promise<void>;
  sendResponse(chatId: string, text: string): Promise<void>;
  completeResponse(chatId: string): Promise<void>;
}

/** Callback to deliver a user message to the bridge. */
export type SendFunc = (chatId: string, text: string) => void;
