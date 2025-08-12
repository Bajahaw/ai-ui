import { BranchingConversation } from "@/lib/conversation";

// Test utility to demonstrate branching functionality
export function createBranchingExample(): BranchingConversation {
  const conv = new BranchingConversation();

  // Create initial conversation
  const userMsg1 = conv.addMessage("Hi there!", "user");
  const assistantMsg1 = conv.addMessage(
    "Hello! How can I help you today?",
    "assistant",
    userMsg1,
  );

  // User asks a question
  const userMsg2 = conv.addMessage(
    "Can you tell me a joke?",
    "user",
    assistantMsg1,
  );

  // Assistant provides first response
  conv.addMessage(
    "Why don't scientists trust atoms? Because they make up everything!",
    "assistant",
    userMsg2,
  );

  // Create alternative responses (branches)
  conv.addBranchMessage(
    "What do you call a bear with no teeth? A gummy bear!",
    "assistant",
    userMsg2,
  );

  const assistantMsg2Alt2 = conv.addBranchMessage(
    "Why did the scarecrow win an award? Because he was outstanding in his field!",
    "assistant",
    userMsg2,
  );

  // User responds to current active branch
  const userMsg3 = conv.addMessage(
    "That's funny! Tell me another one.",
    "user",
    assistantMsg2Alt2,
  );

  // Assistant provides another joke
  conv.addMessage(
    "Why don't eggs tell jokes? They'd crack each other up!",
    "assistant",
    userMsg3,
  );

  // Create another branch for the follow-up
  conv.addBranchMessage(
    "What do you call a fake noodle? An impasta!",
    "assistant",
    userMsg3,
  );

  console.log("Created branching conversation with structure:");
  console.log("User: Hi there!");
  console.log("Assistant: Hello! How can I help you today?");
  console.log("User: Can you tell me a joke?");
  console.log("Assistant (3 branches):");
  console.log("  1. Why don't scientists trust atoms?...");
  console.log("  2. What do you call a bear with no teeth?...");
  console.log("  3. Why did the scarecrow win an award?... (active)");
  console.log("User: That's funny! Tell me another one.");
  console.log("Assistant (2 branches):");
  console.log("  1. Why don't eggs tell jokes?...");
  console.log("  2. What do you call a fake noodle?... (active)");

  return conv;
}

// Test function to demonstrate navigation
export function testBranchNavigation() {
  const conv = createBranchingExample();

  console.log("\n--- Testing Branch Navigation ---");

  // Get current active path
  const activePath = conv.getActivePath();
  console.log("Current active path length:", activePath.length);

  // Get the last message (should be the alternative joke)
  const lastMessage = activePath[activePath.length - 1];
  console.log("Current active message:", lastMessage.content);

  // Check if it has branches
  const hasBranches = conv.hasBranches(lastMessage.id);
  console.log("Last message has branches:", hasBranches);

  if (hasBranches) {
    const currentIndex = conv.getCurrentBranchIndex(lastMessage.id);
    const totalBranches = conv.getTotalBranches(lastMessage.id);
    console.log(`Current branch: ${currentIndex + 1} of ${totalBranches}`);

    // Navigate to previous branch
    console.log("Navigating to previous branch...");
    const prevBranchId = conv.goToPreviousBranch(lastMessage.id);
    if (prevBranchId) {
      const newActivePath = conv.getActivePath();
      const newLastMessage = newActivePath[newActivePath.length - 1];
      console.log("New active message:", newLastMessage.content);
    }
  }

  return conv;
}

// Example usage in console
export function runBranchingDemo() {
  console.log("🌟 Branching Conversation Demo 🌟");
  const conv = testBranchNavigation();

  console.log("\n--- Full Conversation State ---");
  const allMessages = conv.getAllMessages();
  Object.values(allMessages).forEach((msg) => {
    const indent = msg.parentId ? "  " : "";
    console.log(`${indent}${msg.role}: ${msg.content} (id: ${msg.id})`);
  });

  return conv;
}
