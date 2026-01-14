# frozen_string_literal: true

module Chatwoot
  module Resources
    class Messages
      def initialize(client, conversation_id)
        @client = client
        @conversation_id = conversation_id
      end

      def list(before: nil)
        params = {}
        params[:before] = before if before

        @client.get("/conversations/#{@conversation_id}/messages", params)
      end

      def create(content:, message_type: "outgoing", private: false, content_attributes: {})
        body = {
          content: content,
          message_type: message_type,
          private: private
        }
        body[:content_attributes] = content_attributes unless content_attributes.empty?

        @client.post("/conversations/#{@conversation_id}/messages", body)
      end

      def delete(message_id)
        @client.delete("/conversations/#{@conversation_id}/messages/#{message_id}")
      end
    end
  end
end
