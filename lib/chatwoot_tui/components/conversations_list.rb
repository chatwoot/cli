# frozen_string_literal: true

require_relative "../styles"

module ChatwootTui
  module Components
    class ConversationsList
      def initialize(client)
        @client = client
        @conversations = []
        @cursor = 0
        @loading = false
        @error = nil
      end

      def fetch_conversations
        @loading = true
        @error = nil
        response = @client.conversations.list(status: "open")
        @conversations = response["data"]["payload"] || []
        @loading = false
      rescue Chatwoot::Error => e
        @error = e.message
        @loading = false
      end

      def move_up
        @cursor = [@cursor - 1, 0].max
      end

      def move_down
        @cursor = [@cursor + 1, @conversations.length - 1].min
      end

      def selected
        @conversations[@cursor]
      end

      def view(height)
        title = Styles.title.render(" Conversations ")

        content = if @loading
          Styles.warning.render("Loading...")
        elsif @error
          Styles.error.render("Error: #{@error}")
        elsif @conversations.empty?
          Styles.muted.render("No open conversations")
        else
          render_list(height - 3)
        end

        help = Styles.muted.render("j/k nav • r refresh")

        "#{title}\n#{content}\n#{help}"
      end

      private

      def render_list(max_items)
        visible_start = [@cursor - (max_items / 2), 0].max
        visible_end = [visible_start + max_items, @conversations.length].min
        visible_start = [visible_end - max_items, 0].max

        @conversations[visible_start...visible_end].map.with_index(visible_start) do |conv, i|
          render_conversation(conv, i == @cursor)
        end.join("\n")
      end

      def render_conversation(conv, selected)
        id = conv["id"]
        meta = conv["meta"] || {}
        sender = meta.dig("sender", "name") || "Unknown"
        inbox = meta.dig("inbox", "name") || ""
        messages_count = conv["messages_count"] || 0

        line = "##{id} #{truncate(sender, 15)}"
        meta_line = "#{inbox} (#{messages_count})"

        if selected
          "#{Styles.selected.render("> #{line}")}\n  #{Styles.muted.render(meta_line)}"
        else
          "#{Styles.normal.render("  #{line}")}\n  #{Styles.muted.render(meta_line)}"
        end
      end

      def truncate(str, max)
        str.length > max ? "#{str[0, max - 1]}…" : str
      end
    end
  end
end
