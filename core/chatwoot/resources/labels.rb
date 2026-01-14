# frozen_string_literal: true

module Chatwoot
  module Resources
    class Labels
      def initialize(client, conversation_id)
        @client = client
        @conversation_id = conversation_id
      end

      def list
        @client.get("/conversations/#{@conversation_id}/labels")
      end

      def add(labels)
        @client.post("/conversations/#{@conversation_id}/labels", { labels: Array(labels) })
      end
    end
  end
end
