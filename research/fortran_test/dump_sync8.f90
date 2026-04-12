! dump_sync8.f90 — Runs Fortran sync8 on a WAV file and dumps candidates
! for comparison with Go sync8.
!
! Compile:
!   cd research/fortran_test
!   gfortran -O2 -o dump_sync8 dump_sync8.f90 \
!     ~/Development/wsjt-wsjtx/lib/fftw3mod.f90 \
!     ~/Development/wsjt-wsjtx/lib/four2a.f90 \
!     ~/Development/wsjt-wsjtx/lib/ft8/sync8.f90 \
!     ~/Development/wsjt-wsjtx/lib/ft8/get_spectrum_baseline.f90 \
!     ~/Development/wsjt-wsjtx/lib/indexx.f90 \
!     -lfftw3f -lm

program dump_sync8

  include '~/Development/wsjt-wsjtx/lib/ft8/ft8_params.f90'

  integer MAXCAND
  parameter (MAXCAND=600)
  integer*2 iwave(NMAX)
  real dd(NMAX)
  real candidate(3,MAXCAND)
  real sbase(NH1)
  integer ihdr(11)
  integer ncand, ios, i
  character*256 wavfile

  ! Read WAV file
  if (command_argument_count() < 1) then
    print*, 'Usage: dump_sync8 <wavfile>'
    stop
  endif
  call get_command_argument(1, wavfile)

  open(10, file=trim(wavfile), access='stream', status='old', iostat=ios)
  if (ios /= 0) then
    print*, 'Cannot open: ', trim(wavfile)
    stop
  endif
  read(10) ihdr
  iwave = 0
  read(10, iostat=ios) iwave
  close(10)
  dd = iwave

  ! Run sync8 matching ft8_decode.f90 pass 1 (syncmin=1.3, nfqso=0, maxcand=600)
  call sync8(dd, NMAX, 200, 2600, 1.3, 0, MAXCAND, candidate, ncand, sbase)

  write(*,'(A,I6)') 'ncand=', ncand
  write(*,'(A)') 'idx     freq       dt      sync'
  do i=1,min(ncand,50)  ! dump first 50
    write(*,'(I4,F10.3,F10.4,F10.4)') i, candidate(1,i), candidate(2,i), candidate(3,i)
  enddo

  ! Also dump candidates near key frequencies
  write(*,'(A)') ''
  write(*,'(A)') '=== Candidates near missing signals ==='
  do i=1,ncand
    ! RA6ABC ~1814 Hz
    if(abs(candidate(1,i)-1814.0).lt.20.0) then
      write(*,'(A,I4,F10.3,F10.4,F10.4)') 'near1814: ', i, candidate(1,i), candidate(2,i), candidate(3,i)
    endif
    ! RV6ASU ~461 Hz
    if(abs(candidate(1,i)-461.0).lt.20.0) then
      write(*,'(A,I4,F10.3,F10.4,F10.4)') 'near461:  ', i, candidate(1,i), candidate(2,i), candidate(3,i)
    endif
    ! UA3LAR ~835 Hz
    if(abs(candidate(1,i)-835.0).lt.20.0) then
      write(*,'(A,I4,F10.3,F10.4,F10.4)') 'near835:  ', i, candidate(1,i), candidate(2,i), candidate(3,i)
    endif
    ! RA1OHX ~2099 Hz (subtraction_needed)
    if(abs(candidate(1,i)-2099.0).lt.20.0) then
      write(*,'(A,I4,F10.3,F10.4,F10.4)') 'near2099: ', i, candidate(1,i), candidate(2,i), candidate(3,i)
    endif
    ! WB9VGJ ~2328 Hz (subtraction_needed)
    if(abs(candidate(1,i)-2328.0).lt.20.0) then
      write(*,'(A,I4,F10.3,F10.4,F10.4)') 'near2328: ', i, candidate(1,i), candidate(2,i), candidate(3,i)
    endif
  enddo

end program dump_sync8
