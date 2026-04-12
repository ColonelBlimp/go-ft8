! Traces OSD internals for candidate 9's LLR values
program dump_osd_trace
  integer, parameter :: N=174, K=91, M=N-K
  real llr(N), absrx(N), rx(N)
  integer*1 hdec(N)
  integer indx(N)
  integer*1 gen(K,N)
  integer*1 genmrb(K,N), g2(N,K)
  integer*1 temp(K), m0(K), c0(N), cw(N)
  integer*1 me(K), ce(N), mi(K), misub(K)
  integer*1 e2sub(M), e2(M)
  integer indices(N), nxor(N)
  integer*1 message91(91), m96(96)
  character*14 c14
  logical first

  ! Build generator
  do i=1,K
    message91=0; message91(i)=1
    if(i.le.77) then
      m96=0; m96(1:91)=message91
      call get_crc14(m96,96,ncrc14)
      write(c14,'(b14.14)') ncrc14
      read(c14,'(14i1)') message91(78:91)
      message91(78:K)=0
    endif
    call encode174_91_nocrc(message91,cw)
    gen(i,:)=cw
  enddo

  ! Read LLR
  open(10,file='llr_cand9.txt',status='old')
  do i=1,N
    read(10,*) idx, llr(i)  ! read only column 1
  enddo
  close(10)

  rx=llr
  hdec=0
  where(rx.ge.0) hdec=1
  absrx=abs(rx)
  call indexx(absrx,N,indx)

  ! Reorder by decreasing reliability
  do i=1,N
    genmrb(1:K,i)=gen(1:K,indx(N+1-i))
    indices(i)=indx(N+1-i)
  enddo

  ! GE with tracing
  npivfail=0
  do id=1,K
    iflag=0
    do icol=id,K+20
      if(genmrb(id,icol).eq.1) then
        iflag=1
        if(icol.ne.id) then
          temp(1:K)=genmrb(1:K,id)
          genmrb(1:K,id)=genmrb(1:K,icol)
          genmrb(1:K,icol)=temp(1:K)
          itmp=indices(id); indices(id)=indices(icol); indices(icol)=itmp
        endif
        do ii=1,K
          if(ii.ne.id.and.genmrb(ii,id).eq.1) genmrb(ii,1:N)=ieor(genmrb(ii,1:N),genmrb(id,1:N))
        enddo
        exit
      endif
    enddo
    if(iflag.eq.0) then
      npivfail=npivfail+1
      write(*,'(A,I4)') 'GE pivot fail at id=',id
    endif
  enddo
  write(*,'(A,I4)') 'GE pivot failures: ', npivfail

  g2=transpose(genmrb)
  hdec=hdec(indices)
  m0=hdec(1:K)
  absrx=absrx(indices)

  call mrbencode91(m0,c0,g2,N,K)
  nxor=ieor(c0,hdec)
  nhardmin=sum(nxor)
  dmin=0.
  do i=1,N; dmin=dmin+nxor(i)*absrx(i); enddo
  write(*,'(A,I4,A,F10.4)') 'Order-0: nhardmin=',nhardmin,' dmin=',dmin

  ! Order-1 search (ndeep=2: nord=1, npre1=1, nt=40, ntheta=10)
  nt=40; ntheta=10
  misub=0; misub(K)=1
  iflag=K
  npassed=0
  do n1=iflag,1,-1
    mi=misub; mi(n1)=1
    me=ieor(m0,mi)
    if(n1.eq.iflag) then
      call mrbencode91(me,ce,g2,N,K)
      e2sub=ieor(ce(K+1:N),hdec(K+1:N))
      e2=e2sub
      nd1kpt=sum(e2sub(1:nt))+1
    else
      e2=ieor(e2sub,g2(K+1:N,n1))
      nd1kpt=sum(e2(1:nt))+2
    endif
    if(nd1kpt.le.ntheta) then
      npassed=npassed+1
      call mrbencode91(me,ce,g2,N,K)
      nxor=ieor(ce,hdec)
      dd=0.
      do i=1,N; dd=dd+nxor(i)*absrx(i); enddo
      nhard=sum(nxor)
      if(dd.lt.dmin) then
        write(*,'(A,I4,A,I4,A,F10.4,A,I4)') '  BETTER n1=',n1,' nhard=',nhard,' dd=',dd,' nd1kpt=',nd1kpt
        dmin=dd; nhardmin=nhard; cw=ce
      endif
    endif
  enddo
  write(*,'(A,I4,A,I4)') 'Order-1: passed=',npassed,' final nhardmin=',nhardmin

  ! CRC check
  cw=cw(indices)
  m96=0; m96(1:77)=cw(1:77); m96(83:96)=cw(78:91)
  call get_crc14(m96,96,nbadcrc)
  write(*,'(A,I4)') 'CRC check: nbadcrc=', nbadcrc

end program

subroutine mrbencode91(me,ce,g2,N,K)
  integer*1 me(K),ce(N),g2(N,K)
  ce=0
  do i=1,K
    if(me(i).eq.1) ce=ieor(ce,g2(1:N,i))
  enddo
end subroutine
