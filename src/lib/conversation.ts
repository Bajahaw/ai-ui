export interface BranchMessage {
  id: string;
  role: "user" | "assistant";
  content: string;
  parentId?: string;
  children: string[];
  timestamp: number;
  status?: "success" | "error" | "pending";
  error?: string;
}

export class BranchingConversation {
  private messages: Record<string, BranchMessage> = {};
  private rootIds: string[] = [];
  private activePath: string[] = []; // Track the full active path instead of just one message

  constructor() {}

  // Generate a unique ID for messages
  private generateId(): string {
    return `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
  }

  // Add a new message to the conversation
  addMessage(
    content: string,
    role: "user" | "assistant",
    parentId?: string,
    status: "success" | "error" | "pending" = "success",
    error?: string,
  ): string {
    const id = this.generateId();
    const message: BranchMessage = {
      id,
      role,
      content,
      parentId,
      children: [],
      timestamp: Date.now(),
      status,
      error,
    };

    this.messages[id] = message;

    if (!parentId) {
      this.rootIds.push(id);
    } else {
      const parent = this.messages[parentId];
      if (parent) {
        parent.children.push(id);
      }
    }

    // Update active path to include this new message
    if (!parentId) {
      this.activePath = [id];
    } else {
      // Find parent in active path and extend from there
      const parentIndex = this.activePath.indexOf(parentId);
      if (parentIndex >= 0) {
        this.activePath = [...this.activePath.slice(0, parentIndex + 1), id];
      } else {
        // Parent not in active path, rebuild path
        this.activePath = this.buildPathToMessage(id);
      }
    }

    return id;
  }

  // Set the active message (changes the current branch)
  setActive(id: string): void {
    if (this.messages[id]) {
      // Rebuild the full path to this message and continue with its linear descendants
      this.activePath = this.buildFullBranchPath(id);
    } else {
      throw new Error(`Unknown message ID: ${id}`);
    }
  }

  // Get the current active path (linear conversation view)
  getActivePath(): BranchMessage[] {
    return this.activePath.map((id) => this.messages[id]).filter(Boolean);
  }

  // Build path from root to a specific message
  private buildPathToMessage(messageId: string): string[] {
    const path: string[] = [];
    let currentId: string | undefined = messageId;

    while (currentId) {
      const msg = this.messages[currentId];
      if (!msg) break;
      path.unshift(currentId);
      currentId = msg.parentId;
    }

    return path;
  }

  // Build full branch path including linear descendants
  private buildFullBranchPath(messageId: string): string[] {
    // First get path to the message
    const pathToMessage = this.buildPathToMessage(messageId);

    // Then extend with linear descendants (no branching)
    let currentId = messageId;
    while (currentId) {
      const message = this.messages[currentId];
      if (!message || message.children.length === 0) {
        break;
      }

      // Only continue if there's exactly one child (linear path)
      if (message.children.length === 1) {
        const childId = message.children[0];
        pathToMessage.push(childId);
        currentId = childId;
      } else {
        // Multiple children means branching - stop here
        break;
      }
    }

    return pathToMessage;
  }

  // Get all branches (alternative messages) at a specific message
  getBranchesAt(messageId: string): BranchMessage[] {
    const message = this.messages[messageId];
    if (!message || !message.parentId) {
      return [];
    }

    const parent = this.messages[message.parentId];
    if (!parent) {
      return [];
    }

    return parent.children
      .map((childId) => this.messages[childId])
      .filter(Boolean);
  }

  // Get the current active message (last message in active path)
  getActiveMessage(): BranchMessage | undefined {
    if (this.activePath.length === 0) return undefined;
    const lastMessageId = this.activePath[this.activePath.length - 1];
    return this.messages[lastMessageId];
  }

  // Get a specific message by ID
  getMessage(id: string): BranchMessage | undefined {
    return this.messages[id];
  }

  // Update a message
  updateMessage(id: string, updates: Partial<BranchMessage>): void {
    const message = this.messages[id];
    if (message) {
      Object.assign(message, updates);
    }
  }

  // Add a branching message (alternative response to existing message)
  addBranchMessage(
    content: string,
    role: "user" | "assistant",
    parentId: string,
    status: "success" | "error" | "pending" = "success",
    error?: string,
  ): string {
    if (!this.messages[parentId]) {
      throw new Error(`Parent message not found: ${parentId}`);
    }

    const newMessageId = this.addMessage(
      content,
      role,
      parentId,
      status,
      error,
    );

    // When adding a branch message, we need to update the active path to this new branch
    this.activePath = this.buildFullBranchPath(newMessageId);

    return newMessageId;
  }

  // Check if a message has branches (multiple children)
  hasBranches(messageId: string): boolean {
    const message = this.messages[messageId];
    if (!message || !message.parentId) return false;

    const parent = this.messages[message.parentId];
    return parent ? parent.children.length > 1 : false;
  }

  // Get the index of current message among its siblings
  getCurrentBranchIndex(messageId: string): number {
    const message = this.messages[messageId];
    if (!message || !message.parentId) return 0;

    const parent = this.messages[message.parentId];
    if (!parent) return 0;

    return parent.children.indexOf(messageId);
  }

  // Get total number of branches for a message
  getTotalBranches(messageId: string): number {
    const message = this.messages[messageId];
    if (!message || !message.parentId) return 1;

    const parent = this.messages[message.parentId];
    return parent ? parent.children.length : 1;
  }

  // Navigate to next branch
  goToNextBranch(messageId: string): string | null {
    const message = this.messages[messageId];
    if (!message || !message.parentId) return null;

    const parent = this.messages[message.parentId];
    if (!parent) return null;

    const currentIndex = parent.children.indexOf(messageId);
    const nextIndex = (currentIndex + 1) % parent.children.length;
    const nextMessageId = parent.children[nextIndex];

    if (nextMessageId && nextMessageId !== messageId) {
      this.activePath = this.buildFullBranchPath(nextMessageId);
      return nextMessageId;
    }

    return null;
  }

  // Navigate to previous branch
  goToPreviousBranch(messageId: string): string | null {
    const message = this.messages[messageId];
    if (!message || !message.parentId) return null;

    const parent = this.messages[message.parentId];
    if (!parent) return null;

    const currentIndex = parent.children.indexOf(messageId);
    const prevIndex =
      currentIndex === 0 ? parent.children.length - 1 : currentIndex - 1;
    const prevMessageId = parent.children[prevIndex];

    if (prevMessageId && prevMessageId !== messageId) {
      this.activePath = this.buildFullBranchPath(prevMessageId);
      return prevMessageId;
    }

    return null;
  }

  // Get all messages (for debugging or advanced operations)
  getAllMessages(): Record<string, BranchMessage> {
    return { ...this.messages };
  }

  // Clear all messages
  clear(): void {
    this.messages = {};
    this.rootIds = [];
    this.activePath = [];
  }

  // Export conversation data
  export(): {
    messages: Record<string, BranchMessage>;
    rootIds: string[];
    activePath: string[];
  } {
    return {
      messages: { ...this.messages },
      rootIds: [...this.rootIds],
      activePath: [...this.activePath],
    };
  }

  // Import conversation data
  import(data: {
    messages: Record<string, BranchMessage>;
    rootIds: string[];
    activePath?: string[];
  }): void {
    this.messages = { ...data.messages };
    this.rootIds = [...data.rootIds];
    this.activePath = data.activePath || [];
  }
}
