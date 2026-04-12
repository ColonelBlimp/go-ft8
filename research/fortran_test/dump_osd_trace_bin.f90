! dump_osd_trace_bin.f90 — Traces OSD internals reading bit-exact bmet from binary.
! Matches osd174_91.f90 with ndeep=2 (nord=1, npre1=1, nt=40, ntheta=10).
program dump_osd_trace_bin
  integer, parameter :: N=174, K=91, M=N-K
  real llr(N), absrx(N), rx(N)
  integer*1 hdec(N), apmask(N)
  integer indx(N)
  integer*1 gen(K,N)
  integer*1 genmrb(K,N), g2(N,K)
  integer*1 temp(K), m0(K), c0(N), cw(N), bestcw(N)
  integer*1 me(K), ce(N), mi(K), misub(K)
  integer*1 e2sub(M), e2(M), nxor(N)
  integer indices(N)
  integer*1 message91(91), m96(96)
  character*14 c14
  real scalefac, dmin, dd, d1, bm
  real bmeta(174), bmetb(174), bmetc(174), bmetd(174)
  integer nhardmin, nd1kpt

  include '/home/mveary/Development/wsjt-wsjtx/lib/ft8/ft8_params.f90'

  scalefac=2.83

  ! Build generator matrix (same as osd174_91.f90 lines 37-65)
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

  ! Read binary bmet
  open(10,file='bmet_cand9.bin',access='stream',status='old')
  read(10) bmeta
  read(10) bmetb
  read(10) bmetc
  read(10) bmetd
  close(10)

  ! Use bmeta (pass 1)
  do i=1,N
    llr(i)=scalefac*bmeta(i)
  enddo

  rx=llr
  hdec=0
  where(rx.ge.0) hdec=1
  absrx=abs(rx)
  apmask=0

  ! Sort by ascending |LLR| (indexx returns ascending order)
  call indexx(absrx,N,indx)

  ! Reorder by decreasing reliability
  do i=1,N
    genmrb(1:K,i)=gen(1:K,indx(N+1-i))
    indices(i)=indx(N+1-i)
  enddo

  ! GE
  do id=1,K
    ifound=0
    do icol=id,K+20
      if(icol.gt.N) exit
      if(genmrb(id,icol).eq.1) then
        ifound=1
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
  enddo

  g2=transpose(genmrb)
  hdec=hdec(indices)
  absrx=absrx(indices)
  apmask=apmask(indices)
  m0=hdec(1:K)

  ! Order-0
  call mrbencode91(m0,c0,g2,N,K)
  nxor=ieor(c0,hdec)
  nhardmin=sum(nxor)
  dmin=0.
  do i=1,N; dmin=dmin+nxor(i)*absrx(i); enddo
  bestcw=c0
  write(*,'(A,I4,A,F12.6)') 'Order-0: nhardmin=',nhardmin,' dmin=',dmin

  ! Dump first 10 absrx values and indices
  write(*,'(A)') 'First 10 reordered absrx (most reliable first):'
  do i=1,10
    write(*,'(A,I4,A,I4,A,F12.8)') '  i=',i,' origIdx=',indices(i),' absrx=',absrx(i)
  enddo
  write(*,'(A)') 'Last 10 reordered absrx (least reliable):'
  do i=N-9,N
    write(*,'(A,I4,A,I4,A,F12.8)') '  i=',i,' origIdx=',indices(i),' absrx=',absrx(i)
  enddo

  ! Order-1 search (ndeep=2: nord=1, npre1=1, nt=40, ntheta=10)
  nt=40; ntheta=10
  misub=0; misub(K)=1
  iflag=K
  npassed=0
  ntotal=0

  do while(iflag.ge.0)
    iend=1
    do n1=iflag,iend,-1
      ntotal=ntotal+1
      mi=misub; mi(n1)=1

      ! AP mask check
      skip=0
      do j=1,K
        if(apmask(j).eq.1.and.mi(j).eq.1) then
          skip=1; exit
        endif
      enddo
      if(skip.eq.1) cycle

      me=ieor(m0,mi)

      if(n1.eq.iflag) then
        call mrbencode91(me,ce,g2,N,K)
        e2sub=ieor(ce(K+1:N),hdec(K+1:N))
        e2=e2sub
        nd1kpt=sum(e2sub(1:nt))+1
        d1=0.
        do j=1,K; d1=d1+(ieor(me(j),hdec(j)))*absrx(j); enddo
      else
        e2=ieor(e2sub,g2(K+1:N,n1))
        nd1kpt=sum(e2(1:nt))+2
      endif

      if(nd1kpt.le.ntheta) then
        npassed=npassed+1
        call mrbencode91(me,ce,g2,N,K)
        nxor=ieor(ce,hdec)

        if(n1.eq.iflag) then
          dd=d1
          do j=1,M; dd=dd+e2sub(j)*absrx(K+j); enddo
        else
          dd=d1+ieor(ce(n1),hdec(n1))*absrx(n1)
          do j=1,M; dd=dd+e2(j)*absrx(K+j); enddo
        endif

        nhard=sum(nxor)
        if(dd.lt.dmin) then
          write(*,'(A,I4,A,I4,A,F12.6,A,I4,A,I4)') &
            '  BETTER n1=',n1,' nhard=',nhard,' dd=',dd, &
            ' nd1kpt=',nd1kpt,' iflag=',iflag
          dmin=dd; nhardmin=nhard; bestcw=ce
        endif
      endif
    enddo

    ! Next pattern
    call nextpat91_local(misub,K,1,iflag)
  enddo

  write(*,'(A,I6,A,I4,A,I4)') 'Order-1: total=',ntotal,' passed=',npassed,' nhardmin=',nhardmin

  ! CRC check on best
  cw=bestcw
  ! Reorder back
  do i=1,N
    ce(indices(i))=cw(i)
  enddo
  m96=0; m96(1:77)=ce(1:77); m96(83:96)=ce(78:91)
  call get_crc14(m96,96,nbadcrc)
  write(*,'(A,I6)') 'CRC: nbadcrc=', nbadcrc

end program

subroutine mrbencode91(me,ce,g2,N,K)
  integer*1 me(K),ce(N),g2(N,K)
  ce=0
  do i=1,K
    if(me(i).eq.1) ce=ieor(ce,g2(1:N,i))
  enddo
end subroutine

subroutine nextpat91_local(mi,k,iorder,iflag)
  integer*1 mi(k)
  integer k, iorder, iflag
  ! Find rightmost movable 1
  do i=k,1,-1
    if(mi(i).eq.1) then
      if(i.eq.1) then
        iflag=-1; return
      endif
      ! Check: can move left?
      if(mi(i-1).eq.0) then
        mi(i)=0; mi(i-1)=1
        ! Pack trailing 1s to the right
        n1=0
        do j=i+1,k
          if(mi(j).eq.1) n1=n1+1
          mi(j)=0
        enddo
        do j=k,k-n1+1,-1
          mi(j)=1
        enddo
        iflag=i-1; return
      endif
    endif
  enddo
  iflag=-1
end subroutine
