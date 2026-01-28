# frozen_string_literal: true

require "lipgloss"

module ChatwootTui
  # Singleton styles to avoid repeated allocations that crash Go runtime
  module Styles
    class << self
      def title
        @title ||= Lipgloss::Style.new
          .bold(true)
          .foreground("#FAFAFA")
          .background("#7D56F4")
          .padding(0, 1)
      end

      def selected
        @selected ||= Lipgloss::Style.new
          .foreground("#FF69B4")
          .bold(true)
      end

      def normal
        @normal ||= Lipgloss::Style.new
          .foreground("#FAFAFA")
      end

      def muted
        @muted ||= Lipgloss::Style.new
          .foreground("#626262")
      end

      def warning
        @warning ||= Lipgloss::Style.new
          .foreground("#FFD700")
      end

      def error
        @error ||= Lipgloss::Style.new
          .foreground("#FF4444")
      end

      def border
        @border ||= Lipgloss::Style.new
          .border(Lipgloss::ROUNDED_BORDER)
          .border_foreground("#7D56F4")
      end

      def focused_border
        @focused_border ||= Lipgloss::Style.new
          .border(Lipgloss::ROUNDED_BORDER)
          .border_foreground("#FF69B4")
      end
    end
  end
end
