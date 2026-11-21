Feature: Solving known instances
  Solution invariants (checked by the independent oracle) and known optima.

  Background:
    Given the server is running with config "fast"

  @FT-SOLVE-01
  Scenario: A perfect tiling uses one sheet without waste
    Given a running simulation for fixture "D04-four-identical" with maxEpochsCount 200
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid
    And the best solution uses 1 sheet
    And the best solution waste is 0

  @FT-SOLVE-02
  Scenario: A perfect tiling that accounts for the saw width
    Given a running simulation for fixture "D05-tiling-kerf-ok" with maxEpochsCount 200
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid
    And the best solution uses 1 sheet

  @FT-SOLVE-03
  Scenario: Saw width prevents a perfect tiling
    Given a running simulation for fixture "D06-kerf-breaks" with maxEpochsCount 50
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid
    And the best solution uses 4 sheets

  @FT-SOLVE-04
  Scenario: Identical parts over several sheets
    Given a running simulation for fixture "D07-identical-multi" with maxEpochsCount 200
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid
    And the best solution uses 3 sheets

  # Three identical parts have only 8 distinct genotypes, fewer than the
  # minimum population, so the population cannot be built today.
  @FT-SOLVE-05 @pending
  Scenario: Parts of exactly the sheet size
    Given a running simulation for fixture "D08-exact-sheet" with maxEpochsCount 50
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid
    And the best solution uses 3 sheets
    And every sheet of the best solution holds a single part

  # Only 2 distinct genotypes exist for 2 identical parts, fewer than the
  # minimum population, so the population cannot be built today.
  @FT-SOLVE-06 @pending
  Scenario: Parts that fit only rotated
    Given a running simulation for fixture "D09-rotation" with maxEpochsCount 50
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid
    And the best solution uses 2 sheets

  @FT-SOLVE-07
  Scenario: Unconstrained layout
    Given a running simulation for fixture "D11-unconstrained" with maxEpochsCount 50
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid

  @FT-SOLVE-08
  Scenario: Fractional dimensions and saw width
    Given a running simulation for fixture "D12-fractional" with maxEpochsCount 50
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid

  # D13 lower bound: 50 800 / 29 768 -> 2 sheets; LB + 1 = 3.
  @FT-SOLVE-09 @quality
  Scenario: A mixed instance gets within one sheet of the lower bound
    Given the server is running with config "quality"
    And a start request from fixture "D13-mixed-20"
    Then at least 4 of 5 runs reach 3 sheets within 3000 epochs

  # The optimal layout stacks 60x50 on 50x40 into a 50x100 column, which needs
  # a rotation on an exact tie (s1 == l2 == 50). The strict ">" tie-break in
  # reduce never rotates on a tie, so the optimum cannot be encoded: S gives
  # 110x50, L gives 60x90. Observed: 5 of 5 runs end at 3 sheets.
  @FT-SOLVE-10 @quality @pending
  Scenario: A guillotine-perfect instance reaches its optimum
    Given the server is running with config "quality"
    And a start request from fixture "D16-guillotine-opt"
    Then at least 4 of 5 runs reach 2 sheets within 3000 epochs

  # D04 has only a few optimal genotypes, so 2 elites can converge to equal fitness.
  @FT-SOLVE-11
  Scenario: The early stop ends a converged run before the epoch limit
    Given a running simulation for fixture "D04-four-identical" with parameters:
      | eliteCount     | 2      |
      | windowSize     | 10     |
      | maxEpochsCount | 100000 |
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And the epoch number is at most 99999

  # With one elite the elite deviation is always 0, so the latched early stop
  # fires at epoch windowSize even though the run could still improve.
  @FT-SOLVE-12
  Scenario: The early stop is not premature
    Given a running simulation for fixture "D13-mixed-20" with parameters:
      | eliteCount     | 1   |
      | windowSize     | 10  |
      | maxEpochsCount | 300 |
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And the epoch number is exactly 300

  @FT-SOLVE-13
  Scenario: The best fitness never gets worse during a run
    Given a running simulation for fixture "D13-mixed-20" with maxEpochsCount 150
    When I wait for job completion within 120 seconds
    Then the job status is "completed"
    And the best fitness never increases
    And the epoch numbers never decrease

  @FT-SOLVE-14 @slow
  Scenario: A medium instance produces valid layouts
    Given a running simulation for fixture "D14-medium-60" with maxEpochsCount 300
    When I wait for job completion within 300 seconds
    Then the job status is "completed"
    And every solution is valid
