ALTER TABLE messages DROP CONSTRAINT messages_chat_id_fkey;
ALTER TABLE messages ADD CONSTRAINT messages_chat_id_fkey
    FOREIGN KEY (chat_id) REFERENCES chats(id);
