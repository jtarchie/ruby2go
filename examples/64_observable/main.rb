# rbs_inline: enabled
# args: --seed 1

require "observer"
require "minitest/autorun"

class WeatherStation
  include Observable

  #: (Integer) -> void
  def temperature=(value)
    changed
    notify_observers(value)
  end
end

class Display
  #: (String, Array[String]) -> void
  def initialize(name, log)
    @name = name
    @log = log
  end

  #: (Integer) -> void
  def update(temp)
    @log << "#{@name}: #{temp}F"
  end
end

class Recorder
  #: (Array[String]) -> void
  def initialize(log)
    @count = 0
    @log = log
  end

  #: (Integer) -> void
  def record(temp)
    @count += 1
    @log << "recorder: reading ##{@count} is #{temp}F"
  end
end

class ObservableTest < Minitest::Test
  #: () -> void
  def setup
    @log = [] #: Array[String]
    @station = WeatherStation.new
    @kitchen = Display.new("kitchen", @log)
    @station.add_observer(Display.new("porch", @log))
    @station.add_observer(@kitchen)
    # A second argument names the method to call instead of `update`.
    @station.add_observer(Recorder.new(@log), :record)
  end

  def test_observers_are_notified_in_order
    assert_equal 3, @station.count_observers
    @station.temperature = 72
    assert_equal ["porch: 72F", "kitchen: 72F", "recorder: reading #1 is 72F"], @log
  end

  def test_delete_observer
    @station.temperature = 72
    @station.delete_observer(@kitchen)
    assert_equal 2, @station.count_observers
    @log.clear
    @station.temperature = 68
    assert_equal ["porch: 68F", "recorder: reading #2 is 68F"], @log
  end

  def test_notify_is_a_no_op_unless_changed
    @station.changed
    assert_equal true, @station.changed?
    @station.changed(false)
    assert_equal false, @station.changed?
    @station.notify_observers(999)
    assert_equal [], @log
  end

  def test_delete_observers
    @station.delete_observers
    assert_equal 0, @station.count_observers
    @station.changed
    @station.notify_observers(1)
    assert_equal [], @log
  end
end
