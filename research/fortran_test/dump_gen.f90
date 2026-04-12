program dump_gen
  include '~/Development/wsjt-wsjtx/lib/ft8/ft8_params.f90'
  integer, parameter :: K=91, N=174
  integer*1 message91(91), cw(N)
  integer*1 m96(96)
  character*14 c14

  ! Row 1: set bit 1
  message91=0
  message91(1)=1
  m96=0
  m96(1:91)=message91
  call get_crc14(m96,96,ncrc14)
  write(c14,'(b14.14)') ncrc14
  read(c14,'(14i1)') message91(78:91)
  message91(78:K)=0   ! K=91, so this zeros 78:91

  call encode174_91_nocrc(message91,cw)
  write(*,'(A)') 'Row 1 of gen (first 20 bits):'
  write(*,'(20I2)') cw(1:20)
  write(*,'(A)') 'Row 1 of gen (last 20 bits):'
  write(*,'(20I2)') cw(155:174)

  ! Row 91: set bit 91
  message91=0
  message91(91)=1
  ! i>77, no CRC computation
  call encode174_91_nocrc(message91,cw)
  write(*,'(A)') 'Row 91 of gen (first 20 bits):'
  write(*,'(20I2)') cw(1:20)
end program
