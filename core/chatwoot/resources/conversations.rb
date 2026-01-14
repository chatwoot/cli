# frozen_string_literal: true

module Chatwoot
  module Resources
    class Conversations
      def initialize(client)
        @client = client
      end

      def list(status: nil, assignee_type: nil, inbox_id: nil, labels: nil, page: 1)
        params = { page: page }
        params[:status] = status if status
        params[:assignee_type] = assignee_type if assignee_type
        params[:inbox_id] = inbox_id if inbox_id
        params[:labels] = Array(labels).join(",") if labels

        @client.get("/conversations", params)
      end

      def find(id)
        @client.get("/conversations/#{id}")
      end

      def create(inbox_id:, contact_id: nil, source_id: nil, additional_attributes: {})
        body = { inbox_id: inbox_id }
        body[:contact_id] = contact_id if contact_id
        body[:source_id] = source_id if source_id
        body[:additional_attributes] = additional_attributes unless additional_attributes.empty?

        @client.post("/conversations", body)
      end

      def update(id, **attributes)
        @client.patch("/conversations/#{id}", attributes)
      end

      def toggle_status(id, status:, snoozed_until: nil)
        body = { status: status }
        body[:snoozed_until] = snoozed_until if snoozed_until

        @client.post("/conversations/#{id}/toggle_status", body)
      end

      def toggle_priority(id, priority:)
        @client.post("/conversations/#{id}/toggle_priority", { priority: priority })
      end

      def assign(id, assignee_id:)
        @client.post("/conversations/#{id}/assignments", { assignee_id: assignee_id })
      end

      def meta(status: nil, inbox_id: nil)
        params = {}
        params[:status] = status if status
        params[:inbox_id] = inbox_id if inbox_id

        @client.get("/conversations/meta", params)
      end
    end
  end
end
