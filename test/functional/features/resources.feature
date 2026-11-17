Feature: Server resources
  Memory and throughput observed from outside the server process.
  Nightly only; each scenario runs on its own server.

  # Measured: 125 MB 30 s after the stop (baseline 9 MB); with GOGC=off the
  # heap is only collected when it nears the 128 MB limit.
  @FT-RSC-01 @resource @pending
  Scenario: Stopping a job releases its memory
    Given a server started with config "defaults"
    And I record the server memory baseline
    And a long-running simulation for fixture "D14-medium-60"
    And I wait 10 seconds
    When I pause the simulation
    And I delete the simulation
    Then the server memory returns to within 20% of the baseline within 30 seconds

  # Measured: +111 MB after 5 cycles. The loop aborts if the server exceeds a
  # 2 GB safety guard.
  @FT-RSC-02 @resource @pending
  Scenario: Repeated start/stop does not leak memory
    Given a server started with config "fast"
    And I record the server memory baseline
    When I start and stop a simulation for fixture "D13-mixed-20" 5 times
    Then every repeated call succeeded
    And the server memory grew by less than 50 MB

  # Measured peak: 75 MB.
  @FT-RSC-03 @resource
  Scenario: A medium job stays within the memory budget
    Given a server started with config "defaults"
    And a long-running simulation for fixture "D14-medium-60"
    Then the server memory stays below 512 MB while I poll for 20 seconds

  # Measured about 1 000 epochs/s on a Ryzen 7 5800X (Windows); the floor
  # leaves room for slower CI machines. Re-baseline per CI host.
  @FT-RSC-04 @resource
  Scenario: Throughput of a mixed instance
    Given a server started with config "fast"
    And a long-running simulation for fixture "D13-mixed-20"
    And the epoch number increases within 10 seconds
    Then the job reaches at least 200 epochs per second

  @FT-RSC-05 @resource @pending
  Scenario: Abandoned jobs are cleaned up
    Given a server started with config "fast"
    And a running simulation for fixture "D13-mixed-20" with maxEpochsCount 5
    And I wait for job completion within 30 seconds
    When I wait 30 seconds
    Then the simulation no longer exists

  @FT-RSC-06 @resource @pending
  Scenario: The number of concurrent jobs is limited
    Given a server started with config "fast"
    When I start 50 long-running simulations for fixture "D13-mixed-20"
    Then at least one start was answered with status 429
