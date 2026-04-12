program dump_osd_order
  integer, parameter :: N=174, K=91
  real llr(N), absrx(N)
  integer indx(N)
  integer*1 hdec(N)
  character*256 line

  ! Read LLR values from file (pass 1 = column 2)
  open(10, file='llr_cand9.txt', status='old')
  do i=1,N
    read(10,'(A)') line
    read(line,*) idx, llr(i)
  enddo
  close(10)

  hdec=0
  where(llr.ge.0) hdec=1
  absrx=abs(llr)
  call indexx(absrx,N,indx)

  write(*,'(A)') '=== Top 20 most reliable bits (Fortran) ==='
  do i=N,N-19,-1
    j=indx(i)
    write(*,'(A,I4,A,I4,A,F10.6,A,I2)') 'rank ',N+1-i,' bit=',j,' |llr|=',absrx(j),' hdec=',hdec(j)
  enddo

  ! Show order-0 message
  write(*,'(A)') ''
  write(*,'(A)') '=== Order-0 m0 (first 20 of k=91 most reliable bits) ==='
  do i=1,20
    j=indx(N+1-i)
    write(*,'(A,I4,A,I4,A,I2)') 'pos ',i,' origbit=',j,' hdec=',hdec(j)
  enddo
end program
