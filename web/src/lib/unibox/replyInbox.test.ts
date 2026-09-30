import { describe, it, expect } from "vitest";
import { answerAddress, replyInboxSender, repliesGoTo } from "./replyInbox";

const inbox = { id: "a", email: "replies@acme.com", send_as_email: "", reply_to: "", status: "active" };
const sender = { id: "b", email: "b@acme.com", send_as_email: "", reply_to: "Replies@Acme.com", status: "active" };
const other = { id: "c", email: "c@acme.com", send_as_email: "", reply_to: "", status: "active" };

describe("reply inbox", () => {
    it("answers from the mailbox that pointed its replies here", () => {
        expect(replyInboxSender([inbox, sender, other], "a", "b")).toBe("b");
    });

    it("keeps the holder when the sender's replies go elsewhere or nowhere", () => {
        expect(replyInboxSender([inbox, sender, other], "a", "c")).toBeNull();
        expect(replyInboxSender([inbox, sender], "a", undefined)).toBeNull();
        expect(replyInboxSender([inbox, sender], "a", "a")).toBeNull();
    });

    it("keeps the holder when the sender can no longer send", () => {
        expect(replyInboxSender([inbox, { ...sender, status: "inactive" }], "a", "b")).toBeNull();
        expect(replyInboxSender([inbox], "a", "b")).toBeNull();
    });

    it("matches the holder's send-as address", () => {
        expect(repliesGoTo({ ...sender, reply_to: "hello@acme.com" }, { ...inbox, send_as_email: "hello@acme.com" })).toBe(true);
    });

    it("says where an answer comes back to", () => {
        expect(answerAddress(sender)).toBe("Replies@Acme.com");
        expect(answerAddress(other)).toBe("c@acme.com");
    });
});
