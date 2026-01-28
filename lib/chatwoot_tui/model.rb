# frozen_string_literal: true

require "bubbletea"
require "lipgloss"
require_relative "../../core/chatwoot"
require_relative "styles"

module ChatwootTui
  class Model
    include Bubbletea::Model

    RATIO = [3, 6, 3].freeze

    def initialize(base_url:, api_key:, account_id:)
      @client = Chatwoot::Client.new(
        base_url: base_url,
        api_key: api_key,
        account_id: account_id
      )
      @width = 80
      @height = 24
      @focused_column = 0
      @conversations_list = Components::ConversationsList.new(@client)
    end

    def init
      @conversations_list.fetch_conversations
      nil
    end

    def update(msg)
      case msg
      when Bubbletea::WindowSizeMessage
        @width = msg.width
        @height = msg.height
        [self, nil]
      when Bubbletea::KeyMessage
        handle_key(msg)
      else
        [self, nil]
      end
    end

    def view
      col_widths = calculate_column_widths(@width)

      left = render_column(@conversations_list.view(@height - 2), col_widths[0], @focused_column == 0)
      center = render_column(center_placeholder, col_widths[1], @focused_column == 1)
      right = render_column(right_placeholder, col_widths[2], @focused_column == 2)

      [left, center, right].join
    end

    private

    def calculate_column_widths(total_width)
      total_ratio = RATIO.sum
      RATIO.map { |r| (total_width * r / total_ratio) - 2 }
    end

    def render_column(content, width, focused)
      style = focused ? Styles.focused_border : Styles.border
      style.width(width).height(@height - 4).render(content)
    end

    def center_placeholder
      "Messages\n(coming soon)"
    end

    def right_placeholder
      "Details\n(coming soon)"
    end

    def handle_key(msg)
      case msg.string
      when "q", "ctrl+c"
        [self, Bubbletea.quit]
      when "tab"
        @focused_column = (@focused_column + 1) % 3
        [self, nil]
      when "shift+tab"
        @focused_column = (@focused_column - 1) % 3
        [self, nil]
      when "up", "k"
        @conversations_list.move_up if @focused_column == 0
        [self, nil]
      when "down", "j"
        @conversations_list.move_down if @focused_column == 0
        [self, nil]
      when "r"
        @conversations_list.fetch_conversations if @focused_column == 0
        [self, nil]
      else
        [self, nil]
      end
    end
  end
end
