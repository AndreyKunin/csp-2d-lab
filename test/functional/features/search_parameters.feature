Feature: Search parameters
  Tournament size, repair weight, flip weight and donor fraction: defaults
  from the configuration, set on start, changed by Configure.

  Background:
    Given the server is running with config "fast"

  # The "fast" profile defaults: tournament 5, repair 3, flip 1, donor 0.5.
  @FT-SP-01
  Scenario: The search parameters come from the configuration
    Given a start request from fixture "D13-mixed-20"
    When I start the simulation
    Then the response status is 201
    And the reported parameter "tournamentSize" is 5
    And the reported parameter "repairWeight" is 3
    And the reported parameter "flipWeight" is 1
    And the reported parameter "donorFraction" is 0.5

  @FT-SP-02
  Scenario: A start request sets the search parameters
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "tournamentSize" is 3
    And the request parameter "repairWeight" is 6
    And the request parameter "flipWeight" is 2
    And the request parameter "donorFraction" is 0.75
    When I start the simulation
    Then the response status is 201
    And the reported parameter "tournamentSize" is 3
    And the reported parameter "repairWeight" is 6
    And the reported parameter "flipWeight" is 2
    And the reported parameter "donorFraction" is 0.75

  @FT-SP-03
  Scenario: A job runs to completion with non-default search parameters
    Given a running simulation for fixture "D13-mixed-20" with parameters:
      | tournamentSize | 2    |
      | repairWeight   | 9    |
      | flipWeight     | 3    |
      | donorFraction  | 1    |
      | maxEpochsCount | 30   |
      | windowSize     | 250  |
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And the epoch number is exactly 30

  @FT-SP-04
  Scenario: The search parameters can be reconfigured on a paused job
    Given a long-running simulation for fixture "D13-mixed-20"
    And I pause the simulation
    When I configure the simulation with:
      | tournamentSize | 7         |
      | repairWeight   | 4         |
      | flipWeight     | 2         |
      | donorFraction  | 0.25      |
      | maxEpochsCount | 100000000 |
      | windowSize     | 250       |
      | cycleCount     | 1000000   |
    Then the response status is 200
    And the job status is "paused"
    And the reported parameter "tournamentSize" is 7
    And the reported parameter "repairWeight" is 4
    And the reported parameter "flipWeight" is 2
    And the reported parameter "donorFraction" is 0.25

  @FT-SP-05
  Scenario Outline: A search parameter of the wrong type is rejected
    Given a long-running simulation for fixture "D13-mixed-20"
    When I send the raw action body '{"action": "configure", "parameters": {"<key>": <value>}}'
    Then the response status is 400
    And the job status is "running"

    Examples:
      | key            | value   |
      | tournamentSize | 2.5     |
      | repairWeight   | 1.5     |
      | flipWeight     | "one"   |
      | donorFraction  | "half"  |

  # The rules are in config/parameters.json (BE-02). The engine still replaces
  # out-of-range values by the defaults, so a job never hangs on a weight of 0.
  @FT-SP-07
  Scenario Outline: Out-of-range search parameters are rejected
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "<key>" is <value>
    When I start the simulation
    Then the response status is 400

    Examples:
      | key            | value |
      | tournamentSize | 0     |
      | tournamentSize | 1     |
      | repairWeight   | 0     |
      | flipWeight     | 0     |
      | flipWeight     | -1    |
      | donorFraction  | 0     |
      | donorFraction  | 1.5   |
