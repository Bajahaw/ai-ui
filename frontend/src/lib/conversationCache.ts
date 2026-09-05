import type { Conversation, Message, WelcomeStats } from "@/lib/api/types";

const DB_NAME = "ai-ui";
const DB_VERSION = 1;
const CONV_STORE = "conversations";
const MSG_STORE = "messages";
const META_STORE = "meta";
const META_KEY = "current";

export type PersistableSnapshot = {
  userId: string;
  conversations: Conversation[];
  messagesById: Record<string, Record<number, Message>>;
  stats?: WelcomeStats;
};

type ConvRecord = {
  cacheKey: string;
  userId: string;
  id: string;
  title?: string;
  createdAt: string;
  updatedAt: string;
};

type MsgRecord = {
  cacheKey: string;
  userId: string;
  conversationId: string;
  messages: Record<number, Message>;
};

type MetaRecord = {
  key: string;
  userId: string;
  stats?: WelcomeStats;
};

function cacheKey(userId: string, conversationId: string): string {
  return `${userId}:${conversationId}`;
}

function toListItem(record: ConvRecord): Conversation {
  return {
    id: record.id,
    userId: record.userId,
    title: record.title,
    createdAt: record.createdAt,
    updatedAt: record.updatedAt,
    messages: {},
  };
}

function toConvRecord(userId: string, conversation: Conversation): ConvRecord {
  return {
    cacheKey: cacheKey(userId, conversation.id),
    userId,
    id: conversation.id,
    title: conversation.title,
    createdAt: conversation.createdAt,
    updatedAt: conversation.updatedAt,
  };
}

type CacheBackend = {
  load(userId?: string): Promise<PersistableSnapshot | null>;
  replaceList(userId: string, conversations: Conversation[]): Promise<void>;
  saveMessages(
    userId: string,
    conversationId: string,
    messages: Record<number, Message>,
  ): Promise<void>;
  saveStats(userId: string, stats: WelcomeStats): Promise<void>;
  deleteConversation(userId: string, conversationId: string): Promise<void>;
  replaceAll(snapshot: PersistableSnapshot): Promise<void>;
  clear(userId?: string): Promise<void>;
};

function createMemoryBackend(): CacheBackend {
  let conversations = new Map<string, ConvRecord>();
  let messages = new Map<string, MsgRecord>();
  let meta: MetaRecord | null = null;

  const resolveUserId = (userId?: string) => userId || meta?.userId;

  return {
    async load(userId) {
      const uid = resolveUserId(userId);
      if (!uid) {
        return null;
      }
      const list: Conversation[] = [];
      const messagesById: Record<string, Record<number, Message>> = {};
      for (const record of conversations.values()) {
        if (record.userId !== uid) continue;
        list.push(toListItem(record));
      }
      for (const record of messages.values()) {
        if (record.userId !== uid) continue;
        messagesById[record.conversationId] = record.messages;
      }
      if (list.length === 0 && Object.keys(messagesById).length === 0 && !meta?.stats) {
        return meta?.userId === uid
          ? { userId: uid, conversations: [], messagesById, stats: meta.stats }
          : null;
      }
      return {
        userId: uid,
        conversations: list,
        messagesById,
        stats: meta?.userId === uid ? meta.stats : undefined,
      };
    },

    async replaceList(userId, list) {
      const keep = new Set(list.map((c) => cacheKey(userId, c.id)));
      for (const key of [...conversations.keys()]) {
        const record = conversations.get(key);
        if (record?.userId === userId && !keep.has(key)) {
          conversations.delete(key);
          messages.delete(key);
        }
      }
      for (const conversation of list) {
        conversations.set(cacheKey(userId, conversation.id), toConvRecord(userId, conversation));
      }
      meta = { key: META_KEY, userId, stats: meta?.userId === userId ? meta.stats : undefined };
    },

    async saveMessages(userId, conversationId, msgs) {
      messages.set(cacheKey(userId, conversationId), {
        cacheKey: cacheKey(userId, conversationId),
        userId,
        conversationId,
        messages: msgs,
      });
      if (!meta || meta.userId !== userId) {
        meta = { key: META_KEY, userId, stats: meta?.stats };
      }
    },

    async saveStats(userId, stats) {
      meta = { key: META_KEY, userId, stats };
    },

    async deleteConversation(userId, conversationId) {
      const key = cacheKey(userId, conversationId);
      conversations.delete(key);
      messages.delete(key);
    },

    async replaceAll(snapshot) {
      await this.replaceList(snapshot.userId, snapshot.conversations);
      messages = new Map(
        [...messages.entries()].filter(([, record]) => record.userId !== snapshot.userId),
      );
      for (const [conversationId, msgs] of Object.entries(snapshot.messagesById)) {
        await this.saveMessages(snapshot.userId, conversationId, msgs);
      }
      if (snapshot.stats) {
        await this.saveStats(snapshot.userId, snapshot.stats);
      }
    },

    async clear(userId) {
      if (!userId) {
        conversations = new Map();
        messages = new Map();
        meta = null;
        return;
      }
      for (const key of [...conversations.keys()]) {
        if (conversations.get(key)?.userId === userId) {
          conversations.delete(key);
        }
      }
      for (const key of [...messages.keys()]) {
        if (messages.get(key)?.userId === userId) {
          messages.delete(key);
        }
      }
      if (meta?.userId === userId) {
        meta = null;
      }
    },
  };
}

function requestToPromise<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
}

function txDone(tx: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error);
    tx.onabort = () => reject(tx.error);
  });
}

function createIdbBackend(): CacheBackend | null {
  if (typeof indexedDB === "undefined") {
    return null;
  }

  let dbPromise: Promise<IDBDatabase> | null = null;

  const open = () => {
    if (!dbPromise) {
      dbPromise = new Promise((resolve, reject) => {
        const request = indexedDB.open(DB_NAME, DB_VERSION);
        request.onupgradeneeded = () => {
          const db = request.result;
          if (!db.objectStoreNames.contains(CONV_STORE)) {
            const store = db.createObjectStore(CONV_STORE, { keyPath: "cacheKey" });
            store.createIndex("userId", "userId", { unique: false });
          }
          if (!db.objectStoreNames.contains(MSG_STORE)) {
            const store = db.createObjectStore(MSG_STORE, { keyPath: "cacheKey" });
            store.createIndex("userId", "userId", { unique: false });
          }
          if (!db.objectStoreNames.contains(META_STORE)) {
            db.createObjectStore(META_STORE, { keyPath: "key" });
          }
        };
        request.onsuccess = () => resolve(request.result);
        request.onerror = () => reject(request.error);
      });
    }
    return dbPromise;
  };

  const readMeta = async (db: IDBDatabase): Promise<MetaRecord | undefined> => {
    const tx = db.transaction(META_STORE, "readonly");
    return requestToPromise(tx.objectStore(META_STORE).get(META_KEY));
  };

  return {
    async load(userId) {
      const db = await open();
      const meta = await readMeta(db);
      const uid = userId || meta?.userId;
      if (!uid) {
        return null;
      }

      const tx = db.transaction([CONV_STORE, MSG_STORE], "readonly");
      const convIndex = tx.objectStore(CONV_STORE).index("userId");
      const msgIndex = tx.objectStore(MSG_STORE).index("userId");
      const [convRecords, msgRecords] = await Promise.all([
        requestToPromise(convIndex.getAll(uid)),
        requestToPromise(msgIndex.getAll(uid)),
      ]);

      const conversations = (convRecords as ConvRecord[]).map(toListItem);
      const messagesById: Record<string, Record<number, Message>> = {};
      for (const record of msgRecords as MsgRecord[]) {
        messagesById[record.conversationId] = record.messages;
      }

      if (
        conversations.length === 0 &&
        Object.keys(messagesById).length === 0 &&
        !meta?.stats
      ) {
        return meta?.userId === uid
          ? { userId: uid, conversations, messagesById, stats: meta.stats }
          : null;
      }

      return {
        userId: uid,
        conversations,
        messagesById,
        stats: meta?.userId === uid ? meta.stats : undefined,
      };
    },

    async replaceList(userId, list) {
      const db = await open();
      const tx = db.transaction([CONV_STORE, MSG_STORE, META_STORE], "readwrite");
      const convStore = tx.objectStore(CONV_STORE);
      const msgStore = tx.objectStore(MSG_STORE);
      const existing = (await requestToPromise(
        convStore.index("userId").getAll(userId),
      )) as ConvRecord[];
      const keep = new Set(list.map((c) => cacheKey(userId, c.id)));
      for (const record of existing) {
        if (!keep.has(record.cacheKey)) {
          convStore.delete(record.cacheKey);
          msgStore.delete(record.cacheKey);
        }
      }
      for (const conversation of list) {
        convStore.put(toConvRecord(userId, conversation));
      }
      const prev = (await requestToPromise(
        tx.objectStore(META_STORE).get(META_KEY),
      )) as MetaRecord | undefined;
      tx.objectStore(META_STORE).put({
        key: META_KEY,
        userId,
        stats: prev?.userId === userId ? prev.stats : undefined,
      });
      await txDone(tx);
    },

    async saveMessages(userId, conversationId, msgs) {
      const db = await open();
      const tx = db.transaction([MSG_STORE, META_STORE], "readwrite");
      tx.objectStore(MSG_STORE).put({
        cacheKey: cacheKey(userId, conversationId),
        userId,
        conversationId,
        messages: msgs,
      } satisfies MsgRecord);
      const prev = (await requestToPromise(
        tx.objectStore(META_STORE).get(META_KEY),
      )) as MetaRecord | undefined;
      if (!prev || prev.userId !== userId) {
        tx.objectStore(META_STORE).put({
          key: META_KEY,
          userId,
          stats: prev?.stats,
        });
      }
      await txDone(tx);
    },

    async saveStats(userId, stats) {
      const db = await open();
      const tx = db.transaction(META_STORE, "readwrite");
      tx.objectStore(META_STORE).put({ key: META_KEY, userId, stats });
      await txDone(tx);
    },

    async deleteConversation(userId, conversationId) {
      const db = await open();
      const tx = db.transaction([CONV_STORE, MSG_STORE], "readwrite");
      const key = cacheKey(userId, conversationId);
      tx.objectStore(CONV_STORE).delete(key);
      tx.objectStore(MSG_STORE).delete(key);
      await txDone(tx);
    },

    async replaceAll(snapshot) {
      await this.replaceList(snapshot.userId, snapshot.conversations);
      const db = await open();
      const tx = db.transaction([MSG_STORE, META_STORE], "readwrite");
      const existing = (await requestToPromise(
        tx.objectStore(MSG_STORE).index("userId").getAll(snapshot.userId),
      )) as MsgRecord[];
      const keep = new Set(
        Object.keys(snapshot.messagesById).map((id) => cacheKey(snapshot.userId, id)),
      );
      for (const record of existing) {
        if (!keep.has(record.cacheKey)) {
          tx.objectStore(MSG_STORE).delete(record.cacheKey);
        }
      }
      for (const [conversationId, msgs] of Object.entries(snapshot.messagesById)) {
        tx.objectStore(MSG_STORE).put({
          cacheKey: cacheKey(snapshot.userId, conversationId),
          userId: snapshot.userId,
          conversationId,
          messages: msgs,
        } satisfies MsgRecord);
      }
      if (snapshot.stats) {
        tx.objectStore(META_STORE).put({
          key: META_KEY,
          userId: snapshot.userId,
          stats: snapshot.stats,
        });
      }
      await txDone(tx);
    },

    async clear(userId) {
      const db = await open();
      if (!userId) {
        const tx = db.transaction([CONV_STORE, MSG_STORE, META_STORE], "readwrite");
        tx.objectStore(CONV_STORE).clear();
        tx.objectStore(MSG_STORE).clear();
        tx.objectStore(META_STORE).clear();
        await txDone(tx);
        return;
      }
      const tx = db.transaction([CONV_STORE, MSG_STORE, META_STORE], "readwrite");
      const convs = (await requestToPromise(
        tx.objectStore(CONV_STORE).index("userId").getAll(userId),
      )) as ConvRecord[];
      const msgs = (await requestToPromise(
        tx.objectStore(MSG_STORE).index("userId").getAll(userId),
      )) as MsgRecord[];
      for (const record of convs) {
        tx.objectStore(CONV_STORE).delete(record.cacheKey);
      }
      for (const record of msgs) {
        tx.objectStore(MSG_STORE).delete(record.cacheKey);
      }
      const meta = (await requestToPromise(
        tx.objectStore(META_STORE).get(META_KEY),
      )) as MetaRecord | undefined;
      if (meta?.userId === userId) {
        tx.objectStore(META_STORE).delete(META_KEY);
      }
      await txDone(tx);
    },
  };
}

const memoryBackend = createMemoryBackend();
let backend: CacheBackend = createIdbBackend() ?? memoryBackend;

async function withBackend<T>(op: (active: CacheBackend) => Promise<T>): Promise<T | undefined> {
  try {
    return await op(backend);
  } catch (error) {
    console.warn("Conversation cache unavailable:", error);
    if (backend !== memoryBackend) {
      backend = memoryBackend;
      try {
        return await op(backend);
      } catch (fallbackError) {
        console.warn("Conversation memory cache failed:", fallbackError);
      }
    }
    return undefined;
  }
}

export const conversationCache = {
  load(userId?: string): Promise<PersistableSnapshot | null> {
    return withBackend((active) => active.load(userId)).then((result) => result ?? null);
  },

  replaceList(userId: string, conversations: Conversation[]): Promise<void> {
    return withBackend((active) => active.replaceList(userId, conversations)).then(() => undefined);
  },

  saveMessages(
    userId: string,
    conversationId: string,
    messages: Record<number, Message>,
  ): Promise<void> {
    return withBackend((active) =>
      active.saveMessages(userId, conversationId, messages),
    ).then(() => undefined);
  },

  saveStats(userId: string, stats: WelcomeStats): Promise<void> {
    return withBackend((active) => active.saveStats(userId, stats)).then(() => undefined);
  },

  deleteConversation(userId: string, conversationId: string): Promise<void> {
    return withBackend((active) =>
      active.deleteConversation(userId, conversationId),
    ).then(() => undefined);
  },

  replaceAll(snapshot: PersistableSnapshot): Promise<void> {
    return withBackend((active) => active.replaceAll(snapshot)).then(() => undefined);
  },

  clear(userId?: string): Promise<void> {
    return withBackend((active) => active.clear(userId)).then(() => undefined);
  },
};

export function resetConversationCacheForTests(): void {
  backend = createMemoryBackend();
}
