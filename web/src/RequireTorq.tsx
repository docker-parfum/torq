import React from "react";
import { Outlet, useLocation, useNavigate } from "react-router-dom";
import { useGetServicesQuery } from "apiSlice";

const RequireTorq = () => {
  const { data: servicesData } = useGetServicesQuery();
  // {
  //   pollingInterval: 30 * 1000, // get status every 30 seconds
  // }
  const navigate = useNavigate();
  const location = useLocation();

  React.useEffect(() => {
    if (servicesData) {
      if (
        servicesData.torqService === undefined ||
        servicesData.torqService.bootTime === undefined ||
        servicesData.torqService.bootTime == ""
      ) {
        navigate("/services", { replace: true, state: location });
      }
    }
  }, [servicesData]);

  return <Outlet />;
};

export default RequireTorq;
