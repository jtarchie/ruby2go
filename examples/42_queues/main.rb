# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

# A worker pool: jobs flow through a Queue, results are summed under a Mutex.
class Pool
  #: (Integer) -> void
  def initialize(size)
    @jobs = Queue.new #: Queue[Integer]
    @lock = Mutex.new
    @total = 0
    @done = 0
    @workers = (1..size).map { Thread.new { work } } #: Array[Thread]
  end

  #: (Integer) -> void
  def submit(n)
    @jobs << n
  end

  #: () -> [Integer, Integer]
  def finish
    @jobs.close
    @workers.each(&:join)
    [@total, @done]
  end

  private

  #: () -> void
  def work
    while (n = @jobs.pop)
      square = n * n
      @lock.synchronize do
        @total += square
        @done += 1
      end
    end
  end
end

class QueuesTest < Minitest::Test
  def test_worker_pool_sums_under_a_mutex
    pool = Pool.new(4)
    (1..1000).each { |i| pool.submit(i) }
    total, done = pool.finish
    assert_equal 1000, done
    assert_equal 333_833_500, total
  end

  # Bounded hand-off: the producer blocks while the SizedQueue is full.
  def test_sized_queue_hands_off_in_order
    belt = SizedQueue.new(2) #: SizedQueue[String]
    producer = Thread.new do
      %w[bolt nut gear spring].each { |part| belt << part }
      belt.close
    end
    received = [] #: Array[String]
    while (part = belt.pop)
      received << part
    end
    producer.join
    assert_equal %w[bolt nut gear spring], received
    assert_equal true, belt.closed?
    assert_equal 2, belt.max

    e = assert_raises(ClosedQueueError) { belt << "late" }
    assert_equal "queue closed", e.message
  end

  # A ConditionVariable gate: the waiter sleeps until signalled.
  def test_condition_variable_gate
    mutex = Mutex.new
    gate = ConditionVariable.new
    open = false
    opened = false
    waiter = Thread.new do
      mutex.synchronize do
        gate.wait(mutex) until open
        opened = true
      end
    end
    mutex.synchronize do
      open = true
      gate.broadcast
    end
    waiter.join
    assert_equal true, opened
  end

  def test_mutex_try_lock_and_unlock
    mutex = Mutex.new
    assert_equal false, mutex.locked?
    assert_equal true, mutex.try_lock
    assert_equal true, mutex.locked?
    mutex.unlock
    e = assert_raises(ThreadError) { mutex.unlock }
    assert_equal "Attempt to unlock a mutex which is not locked", e.message
  end
end
